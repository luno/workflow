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

type TimeoutRecord struct {
	ID           int64
	WorkflowName string
	ForeignID    string
	RunID        string
	Status       int
	Completed    bool
	ExpireAt     time.Time
	CreatedAt    time.Time
}

// pollTimeouts attempts to find the very next
func pollTimeouts[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	status Status,
	timeouts timeouts[Type, Status],
	processName string,
	pollingFrequency time.Duration,
	pauseAfterErrCount int,
) error {
	updateFn := newUpdater[Type, Status](w.recordStore.Lookup, w.recordStore.Store, w.statusGraph, w.clock)
	store := w.recordStore.Store

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		expiredTimeouts, err := w.timeoutStore.ListValid(ctx, w.Name, int(status), w.clock.Now())
		if err != nil {
			return err
		}

		for _, expiredTimeout := range expiredTimeouts {
			r, err := w.recordStore.Latest(ctx, expiredTimeout.WorkflowName, expiredTimeout.ForeignID)
			if err != nil {
				return err
			}

			if r.Status != int(status) || r.RunState.Finished() {
				// Object has been updated already. Mark timeout as cancelled as it is no longer valid.
				err = w.timeoutStore.Cancel(ctx, expiredTimeout.ID)
				if err != nil {
					return err
				}

				// Continue to next expired timeout
				continue
			}

			if r.RunState.Stopped() {
				if w.debugMode {
					log.Info(ctx, "Skipping processing of timeout of stopped workflow record", j.MKV{
						"workflow":       r.WorkflowName,
						"run_id":         r.RunID,
						"foreign_id":     r.ForeignID,
						"process_name":   processName,
						"current_status": r.Status,
						"run_state":      r.RunState.String(),
					})
				}

				// Continue to next expired timeout
				continue
			}

			for _, config := range timeouts.transitions {
				t0 := w.clock.Now()
				err = processTimeout(ctx, w, config, r, expiredTimeout, w.timeoutStore.Complete, store, updateFn, processName, pauseAfterErrCount)
				if err != nil {
					metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t0).Seconds())
					return err
				}

				metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t0).Seconds())
			}
		}

		err = wait(ctx, pollingFrequency)
		if err != nil {
			return err
		}
	}
}

type completeFunc func(ctx context.Context, id int64) error

func processTimeout[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	config timeout[Type, Status],
	r *WireRecord,
	timeout TimeoutRecord,
	completeFn completeFunc,
	store storeFunc,
	updater updater[Type, Status],
	processName string,
	pauseAfterErrCount int,
) error {
	record, err := buildConsumableRecord[Type, Status](store, r)
	if err != nil {
		return err
	}

	next, err := config.TimeoutFunc(ctx, record, w.clock.Now())
	if err != nil {
		// Only keep track of errors if we need to
		if pauseAfterErrCount > 0 {
			count := w.errorCounter.Add(err, processName, record.RunID)
			if count >= pauseAfterErrCount {
				originalErr := err
				_, err := record.Pause(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to pause record after exceeding allowed error count", j.MKV{
						"workflow_name":      record.WorkflowName,
						"foreign_id":         record.ForeignID,
						"current_status":     record.Status.String(),
						"current_status_int": record.Status,
					})
				}

				// Record paused - now clear the error counter.
				w.errorCounter.Clear(originalErr, processName, record.RunID)
				return nil
			}
		}
		return errors.Wrap(err, "failed to process timeout", j.MKV{
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

		metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "next value specified skip").Inc()
		return nil
	}

	err = updater(ctx, Status(timeout.Status), next, record)
	if err != nil {
		return err
	}

	// Mark timeout as having been executed (aka completed) only in the case that true is returned.
	return completeFn(ctx, timeout.ID)
}

type timeouts[Type any, Status StatusType] struct {
	pollingFrequency   time.Duration
	errBackOff         time.Duration
	lagAlert           time.Duration
	pauseAfterErrCount int
	transitions        []timeout[Type, Status]
}

type timeout[Type any, Status StatusType] struct {
	TimerFunc   TimerFunc[Type, Status]
	TimeoutFunc TimeoutFunc[Type, Status]
}

func timeoutPoller[Type any, Status StatusType](w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status]) {
	role := makeRole(w.Name, fmt.Sprintf("%v", int(status)), "timeout-consumer")
	// readableRole can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeout store, or offset store.
	processName := makeRole(status.String(), "timeout-consumer")

	errBackOff := w.defaultOpts.errBackOff
	if timeouts.errBackOff.Nanoseconds() != 0 {
		errBackOff = timeouts.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if timeouts.pollingFrequency.Nanoseconds() != 0 {
		pollingFrequency = timeouts.pollingFrequency
	}

	pauseAfterErrCount := w.defaultOpts.pauseAfterErrCount
	if timeouts.pauseAfterErrCount != 0 {
		pauseAfterErrCount = timeouts.pauseAfterErrCount
	}

	w.run(role, processName, func(ctx context.Context) error {
		err := pollTimeouts(ctx, w, status, timeouts, processName, pollingFrequency, pauseAfterErrCount)
		if err != nil {
			return err
		}

		return nil
	}, errBackOff)
}

func timeoutAutoInserterConsumer[Type any, Status StatusType](
	w *Workflow[Type, Status],
	status Status,
	timeouts timeouts[Type, Status],
) {
	role := makeRole(w.Name, fmt.Sprintf("%v", int(status)), "timeout-auto-inserter-consumer")
	processName := makeRole(status.String(), "timeout-auto-inserter-consumer")

	pauseAfterErrCount := w.defaultOpts.pauseAfterErrCount
	if timeouts.pauseAfterErrCount != 0 {
		pauseAfterErrCount = timeouts.pauseAfterErrCount
	}

	errBackOff := w.defaultOpts.errBackOff
	if timeouts.errBackOff.Nanoseconds() != 0 {
		errBackOff = timeouts.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if timeouts.pollingFrequency.Nanoseconds() != 0 {
		pollingFrequency = timeouts.pollingFrequency
	}

	lagAlert := w.defaultOpts.lagAlert
	if timeouts.lagAlert.Nanoseconds() != 0 {
		lagAlert = timeouts.lagAlert
	}

	w.run(role, processName, func(ctx context.Context) error {
		consumerFunc := func(ctx context.Context, r *Record[Type, Status]) (Status, error) {
			for _, config := range timeouts.transitions {
				expireAt, err := config.TimerFunc(ctx, r, w.clock.Now())
				if err != nil {
					return 0, err
				}

				if expireAt.IsZero() {
					// Ignore and evaluate the next
					continue
				}

				err = w.timeoutStore.Create(ctx, r.WorkflowName, r.ForeignID, r.RunID, int(status), expireAt)
				if err != nil {
					return 0, err
				}
			}

			// Never update status even when successful
			return 0, nil
		}

		topic := Topic(w.Name, int(status))
		consumerStream, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer consumerStream.Close()

		return consumeForever(ctx, w, consumerFunc, 0, lagAlert, pauseAfterErrCount, consumerStream, status, processName, 1, 1)
	}, errBackOff)
}

// TimerFunc exists to allow the specification of when the timeout should expire dynamically. If not time is set then a
// timeout will not be created and the event will be skipped. If the time is set then a timeout will be created and
// once expired TimeoutFunc will be called. Any non-nil error will be retried with backoff.
type TimerFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], now time.Time) (time.Time, error)

// TimeoutFunc runs once the timeout has expired which is set by TimerFunc. If false is returned with a nil error
// then the timeout is skipped and not retried at a later date. If a non-nil error is returned the TimeoutFunc will be
// called again until a nil error is returned. If true is returned with a nil error then the provided record and any
// modifications made to it will be stored and the status updated - continuing the workflow.
type TimeoutFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], now time.Time) (Status, error)
