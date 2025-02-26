package workflow

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
	"github.com/luno/workflow/internal/outboxpb"
)

func outboxConsumer[Type any, Status StatusType](w *Workflow[Type, Status], config outboxConfig) {
	role := makeRole(w.Name(), "outbox", "consumer")
	processName := makeRole("outbox", "consumer")

	errBackOff := w.outboxConfig.errBackOff
	if config.errBackOff > 0 {
		errBackOff = config.errBackOff
	}

	pollingFrequency := w.outboxConfig.pollingFrequency
	if config.pollingFrequency > 0 {
		pollingFrequency = config.pollingFrequency
	}

	lagAlert := w.outboxConfig.lagAlert
	if config.lagAlert > 0 {
		lagAlert = config.lagAlert
	}

	w.run(role, processName, func(ctx context.Context) error {
		return purgeOutbox[Type, Status](
			ctx,
			w.Name(),
			processName,
			w.recordStore,
			w.eventStreamer,
			w.clock,
			pollingFrequency,
			lagAlert,
			config.limit,
		)
	}, errBackOff)
}

func defaultOutboxConfig() outboxConfig {
	return outboxConfig{
		errBackOff:       defaultOutboxErrBackOff,
		pollingFrequency: defaultOutboxPollingFrequency,
		lagAlert:         defaultOutboxLagAlert,
		limit:            1000,
	}
}

type outboxConfig struct {
	errBackOff       time.Duration
	pollingFrequency time.Duration
	lagAlert         time.Duration
	limit            int64
}

func WithOutboxPollingFrequency(d time.Duration) BuildOption {
	return func(bo *buildOptions) {
		bo.outboxConfig.pollingFrequency = d
	}
}

func WithOutboxErrBackoff(d time.Duration) BuildOption {
	return func(bo *buildOptions) {
		bo.outboxConfig.errBackOff = d
	}
}

func WithOutboxLookupLimit(limit int64) BuildOption {
	return func(bo *buildOptions) {
		bo.outboxConfig.limit = limit
	}
}

func WithOutboxLagAlert(d time.Duration) BuildOption {
	return func(bo *buildOptions) {
		bo.outboxConfig.lagAlert = d
	}
}

func purgeOutbox[Type any, Status StatusType](
	ctx context.Context,
	workflowName string,
	processName string,
	recordStore RecordStore,
	stream EventStreamer,
	clock clock.Clock,
	pollingFrequency time.Duration,
	lagAlert time.Duration,
	lookupLimit int64,
) error {
	events, err := recordStore.ListOutboxEvents(ctx, workflowName, lookupLimit)
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

		headers := make(map[Header]string)
		for k, v := range outboxRecord.Headers {
			headers[Header(k)] = v
		}

		foreignID := outboxRecord.RunId
		eventType := int(outboxRecord.Type)

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(workflowName, processName, e.CreatedAt, lagAlert, clock)

		t0 := clock.Now()
		topic := headers[HeaderTopic]
		producer, err := stream.NewSender(ctx, topic)
		if err != nil {
			return err
		}

		err = producer.Send(ctx, foreignID, eventType, headers)
		if err != nil {
			return err
		}

		err = recordStore.DeleteOutboxEvent(ctx, e.ID)
		if err != nil {
			return err
		}

		// Push the time it took to create the producer, send the event, and delete the outbox entry.
		metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t0).Seconds())
	}

	return nil
}
