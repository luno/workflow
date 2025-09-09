package memrecordstore

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/luno/workflow"
	"github.com/luno/workflow/internal/outboxpb"
)

type (
	OutboxLister  func(limit int64) ([]workflow.OutboxEvent, error)
	OutboxDeleter func(ctx context.Context, id string) error
)

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
			time.Sleep(time.Second)
			continue
		}
	}

	return ctx.Err()
}

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
	for _, e := range events {
		var outboxRecord outboxpb.OutboxRecord
		err := proto.Unmarshal(e.Data, &outboxRecord)
		if err != nil {
			return err
		}

		headers := make(map[workflow.Header]string)
		for k, v := range outboxRecord.Headers {
			headers[workflow.Header(k)] = v
		}

		foreignID := outboxRecord.RunId
		eventType := int(outboxRecord.Type)

		topic := headers[workflow.HeaderTopic]
		producer, err := stream.NewSender(ctx, topic)
		if err != nil {
			return err
		}

		err = producer.Send(ctx, foreignID, eventType, headers)
		if err != nil {
			return err
		}

		err = deleter(ctx, e.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}

	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
