package workflow

import (
	"context"
	"fmt"
	"strconv"
	"time"

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

// pollTimeouts attempts to find the very next expired timeout and execute it
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
				w.logger.maybeDebug(ctx, "Skipping processing of timeout of stopped workflow record", map[string]string{
					"workflow":       r.WorkflowName,
					"run_id":         r.RunID,
					"foreign_id":     r.ForeignID,
					"process_name":   processName,
					"current_status": strconv.FormatInt(int64(r.Status), 10),
					"run_state":      r.RunState.String(),
				})

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
	record *Record,
	timeout TimeoutRecord,
	completeFn completeFunc,
	store storeFunc,
	updater updater[Type, Status],
	processName string,
	pauseAfterErrCount int,
) error {
	run, err := buildRun[Type, Status](store, record)
	if err != nil {
		return err
	}

	next, err := config.TimeoutFunc(ctx, run, w.clock.Now())
	if err != nil {
		_, err := maybePause(ctx, pauseAfterErrCount, w.errorCounter, err, processName, run, w.logger)
		if err != nil {
			return fmt.Errorf("pause error: %v, meta: %v", err, map[string]string{
				"run_id":     record.RunID,
				"foreign_id": record.ForeignID,
			})
		}

		return nil
	}

	if skipUpdate(next) {
		w.logger.maybeDebug(ctx, "skipping update", map[string]string{
			"description":   skipUpdateDescription(next),
			"workflow_name": w.Name,
			"foreign_id":    run.ForeignID,
			"run_id":        run.RunID,
			"run_state":     run.RunState.String(),
			"record_status": run.Status.String(),
		})

		metrics.ProcessSkippedEvents.WithLabelValues(w.Name, processName, "next value specified skip").Inc()
		return nil
	}

	err = updater(ctx, Status(timeout.Status), next, run)
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
	role := makeRole(w.Name, strconv.FormatInt(int64(status), 10), "timeout-consumer")
	// readableRole can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeout store, or offset store.
	processName := makeRole(status.String(), "timeout-consumer")

	errBackOff := w.defaultOpts.errBackOff
	if timeouts.errBackOff > 0 {
		errBackOff = timeouts.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if timeouts.pollingFrequency > 0 {
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
	role := makeRole(w.Name, strconv.FormatInt(int64(status), 10), "timeout-auto-inserter-consumer")
	processName := makeRole(status.String(), "timeout-auto-inserter-consumer")

	pauseAfterErrCount := w.defaultOpts.pauseAfterErrCount
	if timeouts.pauseAfterErrCount != 0 {
		pauseAfterErrCount = timeouts.pauseAfterErrCount
	}

	errBackOff := w.defaultOpts.errBackOff
	if timeouts.errBackOff > 0 {
		errBackOff = timeouts.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if timeouts.pollingFrequency > 0 {
		pollingFrequency = timeouts.pollingFrequency
	}

	lagAlert := w.defaultOpts.lagAlert
	if timeouts.lagAlert > 0 {
		lagAlert = timeouts.lagAlert
	}

	w.run(role, processName, func(ctx context.Context) error {
		consumerFunc := func(ctx context.Context, r *Run[Type, Status]) (Status, error) {
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
type TimerFunc[Type any, Status StatusType] func(ctx context.Context, r *Run[Type, Status], now time.Time) (time.Time, error)

// TimeoutFunc runs once the timeout has expired which is set by TimerFunc. If false is returned with a nil error
// then the timeout is skipped and not retried at a later date. If a non-nil error is returned the TimeoutFunc will be
// called again until a nil error is returned. If true is returned with a nil error then the provided record and any
// modifications made to it will be stored and the status updated - continuing the workflow.
type TimeoutFunc[Type any, Status StatusType] func(ctx context.Context, r *Run[Type, Status], now time.Time) (Status, error)
