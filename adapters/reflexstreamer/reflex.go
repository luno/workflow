package reflexstreamer

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
)

func New(writer, reader *sql.DB, table *rsql.EventsTable, cursorStore reflex.CursorStore) workflow.EventStreamer {
	return &constructor{
		writer:      writer,
		reader:      reader,
		eventsTable: table,
		cursorStore: cursorStore,
	}
}

type constructor struct {
	writer            *sql.DB
	reader            *sql.DB
	stream            reflex.StreamFunc
	eventsTable       *rsql.EventsTable
	cursorStore       reflex.CursorStore
	registerGapFiller sync.Once
}

func (c *constructor) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	return &Producer{
		topic:       topic,
		writer:      c.writer,
		eventsTable: c.eventsTable,
	}, nil
}

type Producer struct {
	topic       string
	writer      *sql.DB
	eventsTable *rsql.EventsTable
}

func (p *Producer) Send(ctx context.Context, runID string, statusType int, headers map[workflow.Header]string) error {
	tx, err := p.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	b, err := json.Marshal(headers)
	if err != nil {
		return err
	}

	notify, err := p.eventsTable.InsertWithMetadata(ctx, tx, runID, EventType(statusType), b)
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

func (p *Producer) Close() error {
	return nil
}

func (c *constructor) NewReceiver(
	ctx context.Context,
	topic string,
	name string,
	opts ...workflow.ReceiverOption,
) (workflow.EventReceiver, error) {
	var copts workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&copts)
	}

	pollFrequency := time.Millisecond * 50
	if copts.PollFrequency > 0 {
		pollFrequency = copts.PollFrequency
	}

	table := c.eventsTable.Clone(rsql.WithEventsBackoff(pollFrequency))

	// Only attempt to fill gaps on one consumer.
	c.registerGapFiller.Do(func() {
		rsql.FillGaps(c.writer, table)
	})

	cursor, err := c.cursorStore.GetCursor(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect cursor")
	}

	streamClient, err := table.ToStream(c.reader)(ctx, cursor)
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
	options      workflow.ReceiverOptions
}

func (c *Consumer) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	for ctx.Err() == nil {
		reflexEvent, err := c.streamClient.Recv()
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to receive event")
		}

		if len(reflexEvent.MetaData) == 0 {
			continue
		}

		headers := make(map[workflow.Header]string)
		err = json.Unmarshal(reflexEvent.MetaData, &headers)
		if err != nil {
			return nil, nil, err
		}

		event := &workflow.Event{
			ID:        reflexEvent.IDInt(),
			ForeignID: reflexEvent.ForeignID,
			Type:      reflexEvent.Type.ReflexType(),
			Headers:   headers,
			CreatedAt: reflexEvent.Timestamp,
		}

		eventID := strconv.FormatInt(event.ID, 10)

		// Skip events that are not related to this topic
		if c.topic != headers[workflow.HeaderTopic] {
			if err := c.cursor.SetCursor(ctx, c.name, eventID); err != nil {
				return nil, nil, errors.Wrap(err, "failed to set cursor", j.MKV{
					"consumer":  c.name,
					"event_id":  reflexEvent.ID,
					"event_fid": reflexEvent.ForeignID,
				})
			}
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

func (c *Consumer) Close() error {
	// Provide new context for flushing of cursor values to underlying store
	err := c.cursor.Flush(context.Background())
	if err != nil {
		return err
	}

	if closer, ok := c.streamClient.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

var _ workflow.EventStreamer = (*constructor)(nil)
