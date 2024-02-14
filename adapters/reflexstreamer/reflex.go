package reflexstreamer

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"

	"github.com/luno/workflow"
)

func New(writer, reader *sql.DB, table *rsql.EventsTableInt, cursorStore reflex.CursorStore) workflow.EventStreamer {
	return &constructor{
		writer:      writer,
		reader:      reader,
		eventsTable: table,
		cursorStore: cursorStore,
	}
}

type constructor struct {
	writer      *sql.DB
	reader      *sql.DB
	stream      reflex.StreamFunc
	eventsTable *rsql.EventsTableInt
	cursorStore reflex.CursorStore
}

func (c constructor) NewProducer(topic string) (workflow.Producer, error) {
	return &Producer{
		topic:       topic,
		writer:      c.writer,
		eventsTable: c.eventsTable,
	}, nil
}

type Producer struct {
	topic       string
	writer      *sql.DB
	eventsTable *rsql.EventsTableInt
}

func (p Producer) Send(ctx context.Context, recordID int64, statusType int, headers map[workflow.Header]string) error {
	tx, err := p.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	b, err := json.Marshal(headers)
	if err != nil {
		return err
	}

	notify, err := p.eventsTable.InsertWithMetadata(ctx, tx, recordID, EventType(statusType), b)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	notify()

	return nil
}

func (p Producer) Close() error {
	return nil
}

func (c constructor) NewConsumer(topic string, name string, opts ...workflow.ConsumerOption) (workflow.Consumer, error) {
	var copts workflow.ConsumerOptions
	for _, opt := range opts {
		opt(&copts)
	}

	pollFrequency := time.Millisecond * 50
	if copts.PollFrequency.Nanoseconds() != 0 {
		pollFrequency = copts.PollFrequency
	}

	table := c.eventsTable.Clone(rsql.WithEventsBackoff(pollFrequency))

	cursor, err := c.cursorStore.GetCursor(context.Background(), name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect cursor")
	}

	streamClient, err := table.ToStream(c.reader)(context.Background(), cursor)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		topic:        topic,
		name:         name,
		cursor:       c.cursorStore,
		reader:       c.reader,
		streamClient: streamClient,
		options:      copts,
	}, nil
}

type Consumer struct {
	topic        string
	name         string
	cursor       reflex.CursorStore
	reader       *sql.DB
	streamClient reflex.StreamClient
	options      workflow.ConsumerOptions
}

func (c Consumer) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	for ctx.Err() == nil {
		reflexEvent, err := c.streamClient.Recv()
		if err != nil {
			return nil, nil, err
		}

		if closer, ok := c.streamClient.(io.Closer); ok {
			defer closer.Close()
		}

		headers := make(map[workflow.Header]string)
		err = json.Unmarshal(reflexEvent.MetaData, &headers)
		if err != nil {
			return nil, nil, err
		}

		event := &workflow.Event{
			ID:        reflexEvent.IDInt(),
			ForeignID: reflexEvent.ForeignIDInt(),
			Type:      reflexEvent.Type.ReflexType(),
			Headers:   headers,
			CreatedAt: reflexEvent.Timestamp,
		}

		// Filter out unwanted events
		if skip := c.options.EventFilter(event); skip {
			continue
		}

		return event, func() error {
			// Increment cursor for consumer only if ack function is called.
			eventID := strconv.FormatInt(event.ID, 10)
			if err := c.cursor.SetCursor(ctx, c.name, eventID); err != nil {
				return errors.Wrap(err, "failed to set cursor", j.MKV{
					"consumer":  c.name,
					"event_id":  reflexEvent.ID,
					"event_fid": reflexEvent.ForeignID,
				})
			}

			return nil
		}, nil
	}

	// If the loop breaks then ctx.Err is non-nil
	return nil, nil, ctx.Err()
}

func (c Consumer) Close() error {
	// Provide new context for flushing of cursor values to underlying store
	err := c.cursor.Flush(context.Background())
	if err != nil {
		return err
	}

	return nil
}

var _ workflow.EventStreamer = (*constructor)(nil)
