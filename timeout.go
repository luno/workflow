package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
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
func pollTimeouts[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status]) error {
	for {
		if ctx.Err() != nil {
			return errors.Wrap(ErrWorkflowShutdown, "")
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
				err = w.timeoutStore.Cancel(ctx, expiredTimeout.WorkflowName, expiredTimeout.ForeignID, expiredTimeout.RunID, expiredTimeout.Status)
				if err != nil {
					return err
				}
			}

			var t Type
			err = Unmarshal(r.Object, &t)
			if err != nil {
				return err
			}

			record := Record[Type, Status]{
				WireRecord: *r,
				Status:     Status(r.Status),
				Object:     &t,
			}

			for _, config := range timeouts.Transitions {
				ok, err := config.TimeoutFunc(ctx, &record, w.clock.Now())
				if err != nil {
					return err
				}

				if ok {
					object, err := Marshal(&t)
					if err != nil {
						return err
					}

					wr := &WireRecord{
						ID:           record.ID,
						WorkflowName: record.WorkflowName,
						ForeignID:    record.ForeignID,
						RunID:        record.RunID,
						Status:       int(config.DestinationStatus),
						IsStart:      false,
						IsEnd:        w.endPoints[config.DestinationStatus],
						Object:       object,
						CreatedAt:    record.CreatedAt,
					}

					err = update(ctx, w.eventStreamerFn, w.recordStore, wr)
					if err != nil {
						return err
					}

					// Mark timeout as having been executed (aka completed) only in the case that true is returned.
					err = w.timeoutStore.Complete(ctx, record.WorkflowName, record.ForeignID, record.RunID, int(record.Status))
					if err != nil {
						return err
					}
				}
			}
		}

		err = wait(ctx, timeouts.PollingFrequency)
		if err != nil {
			return err
		}
	}
}

type timeouts[Type any, Status StatusType] struct {
	PollingFrequency time.Duration
	ErrBackOff       time.Duration
	Transitions      []timeout[Type, Status]
}

type timeout[Type any, Status StatusType] struct {
	DestinationStatus Status
	TimerFunc         TimerFunc[Type, Status]
	TimeoutFunc       TimeoutFunc[Type, Status]
}

func timeoutPoller[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status]) {
	role := makeRole(w.Name, fmt.Sprintf("%v", int(status)), "timeout-consumer")

	w.run(role, func(ctx context.Context) error {
		return pollTimeouts(ctx, w, status, timeouts)
	}, timeouts.ErrBackOff)
}

func timeoutAutoInserterConsumer[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, timeouts timeouts[Type, Status]) {
	role := makeRole(w.Name, fmt.Sprintf("%v", int(status)), "timeout-auto-inserter-consumer")

	w.run(role, func(ctx context.Context) error {
		consumerFunc := func(ctx context.Context, r *Record[Type, Status]) (bool, error) {
			for _, config := range timeouts.Transitions {
				expireAt, err := config.TimerFunc(ctx, r, w.clock.Now())
				if err != nil {
					return false, err
				}

				if expireAt.IsZero() {
					// Ignore and evaluate the next
					continue
				}

				err = w.timeoutStore.Create(ctx, r.WorkflowName, r.ForeignID, r.RunID, int(status), expireAt)
				if err != nil {
					return false, err
				}
			}

			// Never update State even when successful
			return false, nil
		}

		return runStepConsumerForever(ctx, w, consumerConfig[Type, Status]{
			PollingFrequency: timeouts.PollingFrequency,
			ErrBackOff:       timeouts.ErrBackOff,
			Consumer:         consumerFunc,
			ParallelCount:    1,
		},
			status,
			role,
			1,
			1,
		)
	}, timeouts.ErrBackOff)
}

// TimerFunc exists to allow the specification of when the timeout should expire dynamically. If not time is set then a
// timeout will not be created and the event will be skipped. If the time is set then a timeout will be created and
// once expired TimeoutFunc will be called. Any non-nil error will be retried with backoff.
type TimerFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], now time.Time) (time.Time, error)

// TimeoutFunc runs once the timeout has expired which is set by TimerFunc. If false is returned with a nil error
// then the timeout is skipped and not retried at a later date. If a non-nil error is returned the TimeoutFunc will be
// called again until a nil error is returned. If true is returned with a nil error then the provided record and any
// modifications made to it will be stored and the status updated - continuing the workflow.
type TimeoutFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], now time.Time) (bool, error)
