package reflexstreamer

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/workflow"
)

// validIdentifier matches safe SQL table/column names (letter or underscore start, alphanumerics/underscores only).
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func New(writer, reader *sql.DB, table *rsql.EventsTable, cursorStore reflex.CursorStore, opts ...Option) workflow.EventStreamer {
	c := &constructor{
		writer:      writer,
		reader:      reader,
		eventsTable: table,
		cursorStore: cursorStore,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Option configures a reflexstreamer constructor.
type Option func(*constructor)

// WithEventsTableName sets the events table name, enabling StreamFromLatest to
// resolve the head cursor at NewReceiver time rather than deferring to the
// first Recv call. Without this option, StreamFromLatest relies on
// reflex.WithStreamFromHead which resolves the head on first Recv, creating a
// race window where events sent between NewReceiver and Recv may be skipped.
func WithEventsTableName(name string) Option {
	return func(c *constructor) {
		c.eventsTableName = name
	}
}

type constructor struct {
	writer            *sql.DB
	reader            *sql.DB
	stream            reflex.StreamFunc
	eventsTable       *rsql.EventsTable
	eventsTableName   string
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
	b, err := json.Marshal(headers)
	if err != nil {
		return err
	}

	notify, err := p.eventsTable.InsertWithMetadata(ctx, p.writer, runID, EventType(statusType), b)
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

	var streamOpts []reflex.StreamOption
	if copts.StreamFromLatest && cursor == "" {
		if c.eventsTableName != "" {
			if !validIdentifier.MatchString(c.eventsTableName) {
				return nil, errors.New("invalid events table name")
			}
			// Resolve head cursor now so events sent between NewReceiver
			// and the first Recv call are not skipped.
			var maxID sql.NullInt64
			err := c.reader.QueryRowContext(ctx,
				"select max(id) from `"+c.eventsTableName+"`",
			).Scan(&maxID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get latest event id")
			}
			if maxID.Valid && maxID.Int64 > 0 {
				cursor = strconv.FormatInt(maxID.Int64, 10)
			}
		} else {
			// Fallback: resolve head on first Recv (has a small race window).
			streamOpts = append(streamOpts, reflex.WithStreamFromHead())
		}
	}

	streamClient, err := table.ToStream(c.reader)(ctx, cursor, streamOpts...)
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
