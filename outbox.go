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
		return purgeOutbox(
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
	disabled         bool
}

// WithOutboxOptions returns a BuildOption that applies the provided OutboxOption functions
// to the outbox configuration used when building a workflow.
//
// The returned option initialises the outbox configuration from the package defaults and
// then applies each supplied OutboxOption in order; the resulting configuration is stored
// on the build options so it is used when registering the workflow's outbox consumer.
func WithOutboxOptions(opts ...OutboxOption) BuildOption {
	return func(bo *buildOptions) {
		options := defaultOutboxConfig()
		for _, set := range opts {
			set(&options)
		}

		bo.outboxConfig = options
	}
}

type OutboxOption func(w *outboxConfig)

func OutboxPollingFrequency(d time.Duration) OutboxOption {
	return func(bo *outboxConfig) {
		bo.pollingFrequency = d
	}
}

func OutboxErrBackOff(d time.Duration) OutboxOption {
	return func(bo *outboxConfig) {
		bo.errBackOff = d
	}
}

func OutboxLookupLimit(limit int64) OutboxOption {
	return func(bo *outboxConfig) {
		bo.limit = limit
	}
}

func OutboxLagAlert(d time.Duration) OutboxOption {
	return func(bo *outboxConfig) {
		bo.lagAlert = d
	}
}

func purgeOutbox(
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
