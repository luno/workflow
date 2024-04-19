package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/log"

	"github.com/luno/workflow/internal/metrics"
)

// ConsumerFunc provides a record that is expected to be modified if the data needs to change. If true is returned with
// a nil error then the record, along with its modifications, will be stored. If false is returned with a nil error then
// the record will not be stored and the event will be skipped and move onto the next event. If a non-nil error is
// returned then the consumer will back off and try again until a nil error occurs or the retry max has been reached
// if a Dead Letter Queue has been configured for the workflow.
type ConsumerFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status]) (Status, error)

type consumerConfig[Type any, Status StatusType] struct {
	pollingFrequency time.Duration
	errBackOff       time.Duration
	consumer         ConsumerFunc[Type, Status]
	parallelCount    int
	lag              time.Duration
	lagAlert         time.Duration
}

func consumer[Type any, Status StatusType](w *Workflow[Type, Status], currentStatus Status, p consumerConfig[Type, Status], shard, totalShards int) {
	role := makeRole(
		w.Name,
		fmt.Sprintf("%v", int(currentStatus)),
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	pollFrequency := w.defaultPollingFrequency
	if p.pollingFrequency.Nanoseconds() != 0 {
		pollFrequency = p.pollingFrequency
	}

	// processName can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeoutstore, or offset store.
	processName := makeRole(
		fmt.Sprintf("%v", currentStatus.String()),
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	topic := Topic(w.Name, int(currentStatus))

	w.run(role, processName, func(ctx context.Context) error {
		consumerStream, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(pollFrequency),
			WithEventFilters(
				shardFilter(shard, totalShards),
				runStateUpdatesFilter(),
			),
		)
		if err != nil {
			return err
		}
		defer consumerStream.Close()

		return consumeForever[Type, Status](ctx, w, p, consumerStream, currentStatus, processName)
	}, p.errBackOff)
}

func consumeForever[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], p consumerConfig[Type, Status], c Consumer, status Status, processName string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e, ack, err := c.Recv(ctx)
		if err != nil {
			return err
		}

		var lag time.Duration
		if p.lag.Nanoseconds() != 0 {
			lag = p.lag
		}

		// Wait until the event's timestamp matches or is older than the specified lag.
		delay := lag - w.clock.Since(e.CreatedAt)
		if lag > 0 && delay > 0 {
			t := w.clock.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C():
				// Resume to consume the event now that it matches or is older than specified lag.
			}
		}

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(w.Name, processName, e.CreatedAt, p.lagAlert, w.clock)

		record, err := w.recordStore.Lookup(ctx, e.ForeignID)
		if errors.Is(err, ErrRecordNotFound) {
			err = ack()
			if err != nil {
				return err
			}

			continue
		} else if err != nil {
			return err
		}

		// Check to see if record is in expected state. If the status isn't in the expected state then skip for
		// idempotency.
		if record.Status != int(status) {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		if record.RunState.Stopped() {
			if w.debugMode {
				log.Info(ctx, "Skipping consumption of stopped workflow record", j.MKV{
					"event_id":       e.ID,
					"workflow":       record.WorkflowName,
					"run_id":         record.RunID,
					"foreign_id":     record.ForeignID,
					"process_name":   processName,
					"current_status": record.Status,
					"run_state":      record.RunState.String(),
				})
			}

			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		t2 := w.clock.Now()
		err = consume(ctx, w, record, p.consumer, ack, safeUpdate, storeAndEmit, processName)
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t2).Seconds())
	}
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

func consume[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	current *WireRecord,
	cf ConsumerFunc[Type, Status],
	ack Ack,
	updater safeUpdater,
	storeAndEmitter storeAndEmitFunc,
	processName string,
) error {
	record, err := buildConsumableRecord[Type, Status](ctx, w.recordStore, storeAndEmitter, current)
	if err != nil {
		return err
	}

	next, err := cf(ctx, record)
	if err != nil {
		return errors.Wrap(err, "failed to consume", j.MKV{
			"workflow_name":      record.WorkflowName,
			"foreign_id":         record.ForeignID,
			"current_status":     record.Status.String(),
			"current_status_int": record.Status,
		})
	}

	if skipUpdate(next) {
		if w.debugMode {
			log.Info(ctx, "skipping update", j.MKV{
				"description":   skipUpdateDescription(next),
				"record_id":     record.ID,
				"workflow_name": w.Name,
				"foreign_id":    record.ForeignID,
				"run_id":        record.RunID,
				"run_state":     record.RunState.String(),
				"record_status": record.Status.String(),
			})
		}

		metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName).Inc()
		return ack()
	}

	b, err := Marshal(&record.Object)
	if err != nil {
		return err
	}

	runState := RunStateRunning
	isEnd := w.endPoints[next]
	if isEnd {
		runState = RunStateCompleted
	}
	wr := &WireRecord{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     runState,
		Status:       int(next),
		Object:       b,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    w.clock.Now(),
	}

	err = updater(ctx, w.recordStore, w.graph, current.Status, wr)
	if err != nil {
		return err
	}

	return ack()
}
