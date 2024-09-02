package workflow

import (
	"context"
	"errors"
	"strconv"
	"time"

	werrors "github.com/luno/workflow/internal/errors"
	"github.com/luno/workflow/internal/metrics"
)

// ConsumerFunc provides a record that is expected to be modified if the data needs to change. If true is returned with
// a nil error then the record, along with its modifications, will be stored. If false is returned with a nil error then
// the record will not be stored and the event will be skipped and move onto the next event. If a non-nil error is
// returned then the consumer will back off and try again until a nil error occurs or the retry max has been reached
// if a Dead Letter Queue has been configured for the workflow.
type ConsumerFunc[Type any, Status StatusType] func(ctx context.Context, r *Run[Type, Status]) (Status, error)

type consumerConfig[Type any, Status StatusType] struct {
	pollingFrequency   time.Duration
	errBackOff         time.Duration
	consumer           ConsumerFunc[Type, Status]
	parallelCount      int
	lag                time.Duration
	lagAlert           time.Duration
	pauseAfterErrCount int
}

func consumer[Type any, Status StatusType](w *Workflow[Type, Status], currentStatus Status, p consumerConfig[Type, Status], shard, totalShards int) {
	role := makeRole(
		w.Name,
		strconv.FormatInt(int64(currentStatus), 10),
		"consumer",
		strconv.FormatInt(int64(shard), 10),
		"of",
		strconv.FormatInt(int64(totalShards), 10),
	)

	// processName can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeoutstore, or offset store.
	processName := makeRole(
		currentStatus.String(),
		"consumer",
		strconv.FormatInt(int64(shard), 10),
		"of",
		strconv.FormatInt(int64(totalShards), 10),
	)

	topic := Topic(w.Name, int(currentStatus))

	errBackOff := w.defaultOpts.errBackOff
	if p.errBackOff > 0 {
		errBackOff = p.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if p.pollingFrequency > 0 {
		pollingFrequency = p.pollingFrequency
	}

	lagAlert := w.defaultOpts.lagAlert
	if p.lagAlert > 0 {
		lagAlert = p.lagAlert
	}

	pauseAfterErrCount := w.defaultOpts.pauseAfterErrCount
	if p.pauseAfterErrCount != 0 {
		pauseAfterErrCount = p.pauseAfterErrCount
	}

	lag := w.defaultOpts.lag
	if p.lag > 0 {
		lag = p.lag
	}

	w.run(role, processName, func(ctx context.Context) error {
		streamConsumer, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer streamConsumer.Close()

		return consumeForever[Type, Status](ctx, w, p.consumer, lag, lagAlert, pauseAfterErrCount, streamConsumer, currentStatus, processName, shard, totalShards)
	}, errBackOff)
}

func consumeForever[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	consumerFunc ConsumerFunc[Type, Status],
	lag time.Duration,
	lagAlert time.Duration,
	pauseAfterErrCount int,
	streamConsumer Consumer,
	status Status,
	processName string,
	shard, totalShards int,
) error {
	updater := newUpdater[Type, Status](w.recordStore.Lookup, w.recordStore.Store, w.statusGraph, w.clock)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e, ack, err := streamConsumer.Recv(ctx)
		if err != nil {
			return err
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
		pushLagMetricAndAlerting(w.Name, processName, e.CreatedAt, lagAlert, w.clock)

		shouldFilter := FilterUsing(e,
			shardFilter(shard, totalShards),
		)
		if shouldFilter {
			metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "filtered out").Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		record, err := w.recordStore.Lookup(ctx, e.ForeignID)
		if errors.Is(err, ErrRecordNotFound) {
			metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "record not found").Inc()
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
			metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "record status not in expected state").Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		if record.RunState.Stopped() {
			if w.debugMode {
				w.logger.Debug(ctx, "Skipping consumption of stopped workflow record", MKV{
					"event_id":       strconv.FormatInt(e.ID, 10),
					"workflow":       record.WorkflowName,
					"run_id":         record.RunID,
					"foreign_id":     record.ForeignID,
					"process_name":   processName,
					"current_status": strconv.FormatInt(int64(record.Status), 10),
					"run_state":      record.RunState.String(),
				})
			}

			metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "record stopped").Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		t2 := w.clock.Now()
		err = consume(ctx, w, record, consumerFunc, ack, w.recordStore.Store, updater, processName, pauseAfterErrCount)
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
	current *Record,
	cf ConsumerFunc[Type, Status],
	ack Ack,
	store storeFunc,
	updater updater[Type, Status],
	processName string,
	pauseAfterErrCount int,
) error {
	record, err := buildConsumableRecord[Type, Status](store, current)
	if err != nil {
		return err
	}

	next, err := cf(ctx, record)
	if err != nil {
		// Only keep track of errors if we need to
		if pauseAfterErrCount > 0 {
			count := w.errorCounter.Add(err, processName, record.RunID)
			if count >= pauseAfterErrCount {
				originalErr := err
				_, err := record.Pause(ctx)
				if err != nil {
					return werrors.WrapWithMeta(err, "failed to pause record after exceeding allowed error count", map[string]string{
						"workflow_name":      record.WorkflowName,
						"foreign_id":         record.ForeignID,
						"current_status":     record.Status.String(),
						"current_status_int": strconv.FormatInt(int64(record.Status), 10),
					})
				}

				// Run paused - now clear the error counter.
				w.errorCounter.Clear(originalErr, processName, record.RunID)
				return ack()
			}
		}

		return werrors.WrapWithMeta(err, "failed to consume", map[string]string{
			"workflow_name":      record.WorkflowName,
			"foreign_id":         record.ForeignID,
			"current_status":     record.Status.String(),
			"current_status_int": strconv.FormatInt(int64(record.Status), 10),
		})
	}

	if skipUpdate(next) {
		if w.debugMode {
			w.logger.Debug(ctx, "skipping update", MKV{
				"description":   skipUpdateDescription(next),
				"record_id":     strconv.FormatInt(record.ID, 10),
				"workflow_name": w.Name,
				"foreign_id":    record.ForeignID,
				"run_id":        record.RunID,
				"run_state":     record.RunState.String(),
				"record_status": record.Status.String(),
			})
		}

		metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "next value specified skip").Inc()
		return ack()
	}

	err = updater(ctx, Status(current.Status), next, record)
	if err != nil {
		return err
	}

	return ack()
}
