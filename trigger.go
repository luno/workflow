package workflow

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luno/jettison/errors"
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
		UpdatedAt: w.clock.Now(),
	}

	err = storeAndEmit(ctx, w.eventStreamerFn, w.recordStore, wr)
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
