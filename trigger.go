package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

func (w *Workflow[Type, Status]) Trigger(ctx context.Context, foreignID string, startingStatus Status, opts ...TriggerOption[Type, Status]) (runID string, err error) {
	return trigger(ctx, w, w.recordStore.Latest, foreignID, startingStatus, opts...)
}

func trigger[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], lookup latestLookup, foreignID string, startingStatus Status, opts ...TriggerOption[Type, Status]) (runID string, err error) {
	if !w.calledRun {
		return "", fmt.Errorf("trigger failed: workflow is not running")
	}

	if !w.statusGraph.IsValid(int(startingStatus)) {
		w.logger.maybeDebug(w.ctx, fmt.Sprintf("ensure %v is configured for workflow: %v", startingStatus, w.Name), map[string]string{})

		return "", fmt.Errorf("trigger failed: status provided is not configured for workflow: %s", startingStatus)
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

	lastRecord, err := lookup(ctx, w.Name, foreignID)
	if errors.Is(err, ErrRecordNotFound) {
		lastRecord = &Record{}
	} else if err != nil {
		return "", err
	}

	// Check that the last run has completed before triggering a new run.
	if lastRecord.RunState.Valid() && !lastRecord.RunState.Finished() {
		// Cannot trigger a new run for this foreignID if there is a workflow in progress.
		return "", ErrWorkflowInProgress
	}

	uid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	runID = uid.String()
	wr := &Record{
		WorkflowName: w.Name,
		ForeignID:    foreignID,
		RunID:        runID,
		RunState:     RunStateInitiated,
		Status:       int(startingStatus),
		Object:       object,
		CreatedAt:    w.clock.Now(),
		UpdatedAt:    w.clock.Now(),
	}

	err = updateWireRecord(ctx, w.recordStore.Store, wr, RunStateUnknown)
	if err != nil {
		return "", err
	}

	return runID, nil
}

type triggerOpts[Type any, Status StatusType] struct {
	initialValue *Type
}

type TriggerOption[Type any, Status StatusType] func(o *triggerOpts[Type, Status])

func WithInitialValue[Type any, Status StatusType](t *Type) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.initialValue = t
	}
}
