package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/luno/workflow"
)

type EventStreamer struct {
	db *sql.DB
}

func NewEventStreamer(db *sql.DB) *EventStreamer {
	return &EventStreamer{db: db}
}

var _ workflow.EventStreamer = (*EventStreamer)(nil)

func (s *EventStreamer) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	return &EventSender{
		db:    s.db,
		topic: topic,
	}, nil
}

func (s *EventStreamer) NewReceiver(
	ctx context.Context,
	topic string,
	name string,
	opts ...workflow.ReceiverOption,
) (workflow.EventReceiver, error) {
	var options workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&options)
	}

	pollFrequency := 10 * time.Millisecond
	if options.PollFrequency > 0 {
		pollFrequency = options.PollFrequency
	}

	return &EventReceiver{
		db:            s.db,
		topic:         topic,
		name:          name,
		options:       options,
		pollFrequency: pollFrequency,
	}, nil
}

type EventSender struct {
	db    *sql.DB
	topic string
}

func (s *EventSender) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}

	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO workflow_events (topic, foreign_id, type, headers)
			VALUES (?, ?, ?, ?)`,
			s.topic, foreignID, statusType, string(headersJSON),
		)
		if err == nil {
			return nil
		}

		// If database is busy, wait a bit and retry
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * 10 * time.Millisecond):
				continue
			}
		}
	}

	return fmt.Errorf("insert event after %d retries: %w", maxRetries, err)
}

func (s *EventSender) Close() error {
	return nil
}

type EventReceiver struct {
	db            *sql.DB
	topic         string
	name          string
	options       workflow.ReceiverOptions
	pollFrequency time.Duration
}

func (r *EventReceiver) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	// Try immediately first, then use ticker
	event, ack, err := r.tryReceive(ctx)
	if err != nil {
		return nil, nil, err
	}
	if event != nil {
		return event, ack, nil
	}

	ticker := time.NewTicker(r.pollFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-ticker.C:
			event, ack, err := r.tryReceive(ctx)
			if err != nil {
				return nil, nil, err
			}
			if event != nil {
				return event, ack, nil
			}
		}
	}
}

func (r *EventReceiver) tryReceive(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	cursor, err := r.getCursor(ctx)
	if err != nil {
		return nil, nil, err
	}

	if r.options.StreamFromLatest && cursor == 0 {
		latestID, err := r.getLatestEventID(ctx)
		if err != nil {
			return nil, nil, err
		}
		err = r.setCursor(ctx, latestID)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, foreign_id, type, headers, created_at
		FROM workflow_events
		WHERE topic = ? AND id > ?
		ORDER BY id ASC
		LIMIT 1`,
		r.topic, cursor,
	)

	var event workflow.Event
	var headersJSON string
	err = row.Scan(&event.ID, &event.ForeignID, &event.Type, &headersJSON, &event.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("scan event: %w", err)
	}

	err = json.Unmarshal([]byte(headersJSON), &event.Headers)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal headers: %w", err)
	}

	ack := func() error {
		return r.setCursor(ctx, event.ID)
	}

	return &event, ack, nil
}

func (r *EventReceiver) getCursor(ctx context.Context) (int64, error) {
	var position int64
	err := r.db.QueryRowContext(ctx, `
		SELECT position FROM workflow_cursors
		WHERE topic = ? AND consumer = ?`,
		r.topic, r.name,
	).Scan(&position)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get cursor: %w", err)
	}
	return position, nil
}

func (r *EventReceiver) setCursor(ctx context.Context, position int64) error {
	const maxRetries = 5
	var err error

	for i := 0; i < maxRetries; i++ {
		_, err = r.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO workflow_cursors (topic, consumer, position, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			r.topic, r.name, position,
		)
		if err == nil {
			return nil
		}

		// If database is busy, wait a bit and retry
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * 10 * time.Millisecond):
				continue
			}
		}
	}

	return fmt.Errorf("set cursor after %d retries: %w", maxRetries, err)
}

func (r *EventReceiver) getLatestEventID(ctx context.Context) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(id), 0) FROM workflow_events WHERE topic = ?`,
		r.topic,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get latest event ID: %w", err)
	}
	return id, nil
}

func (r *EventReceiver) Close() error {
	return nil
}
