package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luno/jettison/errors"
	"github.com/robfig/cron/v3"
	"k8s.io/utils/clock"
)

func (w *Workflow[Type, Status]) Trigger(ctx context.Context, foreignID string, startingStatus Status, opts ...TriggerOption[Type, Status]) (runID string, err error) {
	if !w.calledRun {
		return "", errors.Wrap(ErrWorkflowNotRunning, "ensure Run() is called before attempting to trigger the workflow")
	}

	_, ok := w.validStatuses[startingStatus]
	if !ok {
		return "", errors.Wrap(ErrStatusProvidedNotConfigured, fmt.Sprintf("ensure %v is configured for workflow: %v", startingStatus, w.Name))
	}

	var o triggerOpts[Type, Status]
	for _, fn := range opts {
		fn(&o)
	}

	var t Type
	if o.initialValue != nil {
		t = *o.initialValue
	}

	object, err := Marshal(&t)
	if err != nil {
		return "", err
	}

	lastRecord, err := w.recordStore.Latest(ctx, w.Name, foreignID)
	if errors.Is(err, ErrRecordNotFound) {
		lastRecord = &WireRecord{}
	} else if err != nil {
		return "", err
	}

	// Check that the last entry for that workflow was a terminal step when entered.
	if lastRecord.RunID != "" && !lastRecord.IsEnd {
		// Cannot trigger a new workflow for this foreignID if there is a workflow in progress
		return "", errors.Wrap(ErrWorkflowInProgress, "")
	}

	uid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	runID = uid.String()
	wr := &WireRecord{
		RunID:        runID,
		WorkflowName: w.Name,
		ForeignID:    foreignID,
		Status:       int(startingStatus),
		// isStart is always true when being stored as the trigger as it is the beginning of the workflow
		IsStart: true,
		// isEnd is always false as there should always be more than one node in the graph so that there can be a
		// transition between statuses / states.
		IsEnd:     false,
		Object:    object,
		CreatedAt: w.clock.Now(),
	}

	err = update(ctx, w.eventStreamerFn, w.recordStore, wr)
	if err != nil {
		return "", err
	}

	return runID, nil
}

func (w *Workflow[Type, Status]) ScheduleTrigger(foreignID string, startingStatus Status, spec string, opts ...TriggerOption[Type, Status]) error {
	if !w.calledRun {
		return errors.Wrap(ErrWorkflowNotRunning, "ensure Run() is called before attempting to trigger the workflow")
	}

	_, ok := w.validStatuses[startingStatus]
	if !ok {
		return errors.Wrap(ErrStatusProvidedNotConfigured, fmt.Sprintf("ensure %v is configured for workflow: %v", startingStatus, w.Name))
	}

	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return err
	}

	role := strings.Join([]string{w.Name, fmt.Sprintf("%v", int(startingStatus)), foreignID, "scheduler", spec}, "-")

	w.run(role, func(ctx context.Context) error {
		latestEntry, err := w.recordStore.Latest(ctx, w.Name, foreignID)
		if errors.Is(err, ErrRecordNotFound) {
			// NoReturnErr: Rather use zero value for lastRunID and use current clock for first run.
			latestEntry = &WireRecord{}
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

		_, err = w.Trigger(ctx, foreignID, startingStatus, opts...)
		if errors.Is(err, ErrWorkflowInProgress) {
			// NoReturnErr: Fallthrough to schedule next workflow as there is already one in progress. If this
			// happens it is likely that we scheduled a workflow and were unable to schedule the next.
			return nil
		} else if err != nil {
			return err
		}

		return nil
	}, w.defaultErrBackOff)

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

type triggerOpts[Type any, Status StatusType] struct {
	initialValue   *Type
	startingStatus int
}

type TriggerOption[Type any, Status StatusType] func(o *triggerOpts[Type, Status])

func WithInitialValue[Type any, Status StatusType](t *Type) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.initialValue = t
	}
}
