package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/workflow/internal/metrics"
)

type Timeout struct {
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
func pollTimeouts[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status], processName string) error {
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

			if r.Status != int(status) {
				// Object has been updated already. Mark timeout as cancelled as it is no longer valid.
				err = w.timeoutStore.Cancel(ctx, expiredTimeout.ID)
				if err != nil {
					return err
				}

				// Continue to next expired timeout
				continue
			}

			for _, config := range timeouts.transitions {
				t0 := w.clock.Now()
				err = processTimeout(ctx, w, config, r, expiredTimeout)
				if err != nil {
					metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t0).Seconds())
					return err
				}

				metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t0).Seconds())
			}
		}

		err = wait(ctx, timeouts.pollingFrequency)
		if err != nil {
			return err
		}
	}
}

func processTimeout[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], config timeout[Type, Status], r *WireRecord, timeout Timeout) error {
	record, err := buildConsumableRecord[Type, Status](ctx, w.recordStore, storeAndEmit, r, w.customDelete)
	if err != nil {
		return err
	}

	next, err := config.TimeoutFunc(ctx, record, w.clock.Now())
	if err != nil {
		return err
	}

	if int(next) != 0 {
		object, err := Marshal(&record.Object)
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
			Object:       object,
			CreatedAt:    record.CreatedAt,
			UpdatedAt:    w.clock.Now(),
		}

		err = safeUpdate(ctx, w.recordStore, w.graph, timeout.Status, wr)
		if err != nil {
			return err
		}

		// Mark timeout as having been executed (aka completed) only in the case that true is returned.
		err = w.timeoutStore.Complete(ctx, timeout.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

type timeouts[Type any, Status StatusType] struct {
	pollingFrequency time.Duration
	errBackOff       time.Duration
	lagAlert         time.Duration
	transitions      []timeout[Type, Status]
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

	w.run(role, processName, func(ctx context.Context) error {
		err := pollTimeouts(ctx, w, status, timeouts, processName)
		if err != nil {
			return err
		}

		return nil
	}, timeouts.errBackOff)
}

func timeoutAutoInserterConsumer[Type any, Status StatusType](w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status]) {
	role := makeRole(w.Name, fmt.Sprintf("%v", int(status)), "timeout-auto-inserter-consumer")
	processName := makeRole(status.String(), "timeout-auto-inserter-consumer")

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
			WithConsumerPollFrequency(timeouts.pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer consumerStream.Close()

		cc := consumerConfig[Type, Status]{
			pollingFrequency: timeouts.pollingFrequency,
			errBackOff:       timeouts.errBackOff,
			consumer:         consumerFunc,
			parallelCount:    1,
			lagAlert:         timeouts.lagAlert,
		}

		return consumeForever(ctx, w, cc, consumerStream, status, processName, 1, 1)
	}, timeouts.errBackOff)
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
