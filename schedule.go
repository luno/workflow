package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/utils/clock"
)

func (w *Workflow[Type, Status]) Schedule(
	foreignID string,
	spec string,
	opts ...ScheduleOption[Type, Status],
) error {
	if !w.calledRun {
		return fmt.Errorf("schedule failed: workflow is not running")
	}

	var options scheduleOpts[Type, Status]
	for _, opt := range opts {
		opt(&options)
	}

	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}

	role := makeRole(w.Name(), foreignID, "scheduler", spec)
	processName := makeRole(foreignID, "scheduler", spec)

	w.launching.Add(1)
	w.run(role, processName, func(ctx context.Context) error {
		latestEntry, err := w.recordStore.Latest(ctx, w.Name(), foreignID)
		if errors.Is(err, ErrRecordNotFound) {
			// NoReturnErr: Rather use zero value for lastRunID and use current clock for first run.
			latestEntry = &Record{}
		} else if err != nil {
			return err
		}

		lastRun := latestEntry.CreatedAt

		// If there is no previous executions of this workflow then schedule the very next from now.
		if lastRun.IsZero() {
			lastRun = w.clock.Now()
		}

		nextRun := schedule.Next(lastRun)
		err = waitUntil(ctx, w.clock, nextRun)
		if err != nil {
			return err
		}

		// If there is a trigger initial value ensure that it is passed down to the trigger function through it's own
		// set of optional functions.
		var tOpts []TriggerOption[Type, Status]
		if options.initialValue != nil {
			tOpts = append(tOpts, WithInitialValue[Type, Status](options.initialValue))
		}

		// If a filter has been provided then allow the ability to skip scheduling when false is returned along with
		// a nil error.
		var shouldTrigger bool
		if options.scheduleFilter != nil {
			ok, err := options.scheduleFilter(ctx)
			if err != nil {
				return err
			}

			shouldTrigger = ok
		} else {
			shouldTrigger = true
		}

		if !shouldTrigger {
			return nil
		}

		_, err = w.Trigger(ctx, foreignID, tOpts...)
		if errors.Is(err, ErrWorkflowInProgress) {
			// NoReturnErr: Fallthrough to schedule next workflow as there is already one in progress. If this
			// happens it is likely that we scheduled a workflow and were unable to schedule the next.
			return nil
		} else if err != nil {
			return err
		}

		return nil
	}, w.defaultOpts.errBackOff)

	return nil
}

func waitUntil(ctx context.Context, clock clock.Clock, until time.Time) error {
	timeDiffAsDuration := until.Sub(clock.Now())

	t := clock.NewTimer(timeDiffAsDuration)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

type scheduleOpts[Type any, Status StatusType] struct {
	initialValue   *Type
	scheduleFilter func(ctx context.Context) (bool, error)
}

type ScheduleOption[Type any, Status StatusType] func(o *scheduleOpts[Type, Status])

func WithScheduleInitialValue[Type any, Status StatusType](t *Type) ScheduleOption[Type, Status] {
	return func(o *scheduleOpts[Type, Status]) {
		o.initialValue = t
	}
}

func WithScheduleFilter[Type any, Status StatusType](
	fn func(ctx context.Context) (bool, error),
) ScheduleOption[Type, Status] {
	return func(o *scheduleOpts[Type, Status]) {
		o.scheduleFilter = fn
	}
}
