package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"google.golang.org/protobuf/proto"
	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
	"github.com/luno/workflow/workflowpb"
)

func outboxConsumer[Type any, Status StatusType](w *Workflow[Type, Status], config outboxConfig, shard, totalShards int) {
	role := makeRole(
		w.Name,
		"outbox",
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	// processName can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeoutstore, or offset store.
	processName := makeRole(
		"outbox",
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	w.run(role, processName, func(ctx context.Context) error {
		return purgeOutbox[Type, Status](ctx, w.Name, processName, w.recordStore, w.eventStreamer, w.clock, config.pollingFrequency, config.lagAlert, config.limit, shard, totalShards)
	}, config.errBackOff)
}

func defaultOutboxConfig() outboxConfig {
	return outboxConfig{
		parallelCount:    1,
		errBackOff:       defaultOutboxErrBackOff,
		pollingFrequency: defaultOutboxPollingFrequency,
		lagAlert:         defaultOutboxLagAlert,
		limit:            1000,
	}
}

type outboxConfig struct {
	parallelCount    int
	errBackOff       time.Duration
	pollingFrequency time.Duration
	lagAlert         time.Duration
	limit            int64
}

type OutboxOption func(o *outboxConfig)

func WithOutboxParallelCount(count int) func(o *outboxConfig) {
	return func(o *outboxConfig) {
		o.parallelCount = count
	}
}

func WithOutboxPollingFrequency(d time.Duration) func(o *outboxConfig) {
	return func(o *outboxConfig) {
		o.pollingFrequency = d
	}
}

func WithOutboxErrBackoff(d time.Duration) func(o *outboxConfig) {
	return func(o *outboxConfig) {
		o.errBackOff = d
	}
}

func WithOutboxLookupLimit(limit int64) func(o *outboxConfig) {
	return func(o *outboxConfig) {
		o.limit = limit
	}
}

func WithOutboxLagAlert(d time.Duration) func(o *outboxConfig) {
	return func(o *outboxConfig) {
		o.lagAlert = d
	}
}

func purgeOutbox[Type any, Status StatusType](
	ctx context.Context,
	workflowName string,
	processName string,
	recordStore RecordStore,
	streamer EventStreamer,
	clock clock.Clock,
	pollingFrequency time.Duration,
	lagAlert time.Duration,
	lookupLimit int64,
	shard int,
	totalShards int,
) error {
	events, err := recordStore.ListOutboxEvents(ctx, workflowName, lookupLimit)
	if err != nil {
		return err
	}

	// Send the events to the EventStreamer.
	for _, e := range events {
		var outboxRecord workflowpb.OutboxRecord
		err := proto.Unmarshal(e.Data, &outboxRecord)
		if err != nil {
			return errors.Wrap(err, "Unable to proto unmarshal outbox record")
		}

		headers := make(map[Header]string)
		for k, v := range outboxRecord.Headers {
			headers[Header(k)] = v
		}

		event := &Event{
			ID:        e.ID,
			ForeignID: outboxRecord.ForeignId,
			Type:      int(outboxRecord.Type),
			Headers:   headers,
			CreatedAt: e.CreatedAt,
		}

		// Exclude events that should not be consumed by this shard instance
		shouldExclude := shardFilter(shard, totalShards)(event)
		if shouldExclude {
			continue
		}

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(workflowName, processName, event.CreatedAt, lagAlert, clock)

		t0 := clock.Now()
		topic := headers[HeaderTopic]
		producer, err := streamer.NewProducer(ctx, topic)
		if err != nil {
			return errors.Wrap(err, "Unable to construct new producer for outbox purging")
		}

		err = producer.Send(ctx, event.ForeignID, event.Type, event.Headers)
		if err != nil {
			return errors.Wrap(err, "Unable to send outbox event to event streamer")
		}

		err = recordStore.DeleteOutboxEvent(ctx, event.ID)
		if err != nil {
			return err
		}

		// Push the time it took to create the producer, send the event, and delete the outbox entry.
		metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t0).Seconds())
	}

	return wait(ctx, pollingFrequency)
}
