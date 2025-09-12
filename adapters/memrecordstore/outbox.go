package memrecordstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/luno/workflow"
	"github.com/luno/workflow/internal/outboxpb"
)

type (
	OutboxLister  func(limit int64) ([]workflow.OutboxEvent, error)
	OutboxDeleter func(ctx context.Context, id string) error
)

// PurgeOutboxForever continuously purges outbox events until the context is cancelled.
// 
// It repeatedly calls purgeOutbox to process available outbox entries. If purgeOutbox
// returns context.Canceled the function exits with nil. On any other error the error
// is logged and the function waits one second (respecting context cancellation) before
// retrying. The function blocks until the provided context is cancelled and always
// returns nil on cancellation.
func PurgeOutboxForever(
	ctx context.Context,
	lister OutboxLister,
	deleter OutboxDeleter,
	stream workflow.EventStreamer,
	logger workflow.Logger,
	pollingFrequency time.Duration,
	lookupLimit int64,
) error {
	for ctx.Err() == nil {
		err := purgeOutbox(
			ctx,
			lister,
			deleter,
			stream,
			pollingFrequency,
			lookupLimit,
		)
		if errors.Is(err, context.Canceled) {
			return nil
		} else if err != nil {
			logger.Error(ctx, err)
			if err := wait(ctx, time.Second); errors.Is(err, context.Canceled) {
				return nil
			}

			continue
		}
	}

	return nil
}

// purgeOutbox processes a single batch of outbox events retrieved via lister.
//
// It requests up to lookupLimit events; if none are returned it waits for
// pollingFrequency (respecting ctx cancellation) and returns the wait result.
// For each event it unmarshals an outboxpb.OutboxRecord from the event data,
// converts protobuf headers to workflow headers, requires the workflow.HeaderTopic
// header to determine the destination topic, and sends the event using an
// EventSender obtained from stream.NewSender. A sender is reused per-topic for
// the duration of the batch and closed before returning. After a successful
// send each event is removed via deleter.
//
// Errors from lister, unmarshalling, missing topic header, creating a sender,
// sending an event, deleting an event, or the wait are returned directly. The
// operation honours context cancellation passed via ctx.
func purgeOutbox(
	ctx context.Context,
	lister OutboxLister,
	deleter OutboxDeleter,
	stream workflow.EventStreamer,
	pollingFrequency time.Duration,
	lookupLimit int64,
) error {
	events, err := lister(lookupLimit)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return wait(ctx, pollingFrequency)
	}

	// Send the events to the EventStreamer.
	// Reuse a sender per topic within this batch to avoid connection churn.
	senders := make(map[string]workflow.EventSender)
	defer func() {
		for _, s := range senders {
			_ = s.Close()
		}
	}()

	for _, e := range events {
		var outboxRecord outboxpb.OutboxRecord
		if err := proto.Unmarshal(e.Data, &outboxRecord); err != nil {
			return fmt.Errorf("unmarshal outbox event %s: %w", e.ID, err)
		}

		headers := make(map[workflow.Header]string, len(outboxRecord.Headers))
		for k, v := range outboxRecord.Headers {
			headers[workflow.Header(k)] = v
		}

		foreignID := outboxRecord.RunId
		eventType := int(outboxRecord.Type)

		topic := headers[workflow.HeaderTopic]
		if topic == "" {
			return fmt.Errorf("outbox event %s missing %q header", e.ID, workflow.HeaderTopic)
		}

		sender, ok := senders[topic]
		if !ok {
			var err error
			sender, err = stream.NewSender(ctx, topic)
			if err != nil {
				return fmt.Errorf("create sender for topic %q: %w", topic, err)
			}

			senders[topic] = sender
		}

		if err := sender.Send(ctx, foreignID, eventType, headers); err != nil {
			return fmt.Errorf("send outbox event %s to topic %q: %w", e.ID, topic, err)
		}

		if err := deleter(ctx, e.ID); err != nil {
			return fmt.Errorf("delete outbox event %s: %w", e.ID, err)
		}
	}

	return nil
}

// wait waits for the given duration or until the context is cancelled.
// If d is zero it returns immediately. If the context is cancelled before
// the timer fires it returns ctx.Err(), otherwise it returns nil.
func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
