package workflow

import (
	"context"
	"fmt"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

type RunState int

const (
	RunStateUnknown              RunState = 0
	RunStateInitiated            RunState = 1
	RunStateRunning              RunState = 2
	RunStatePaused               RunState = 3
	RunStateCancelled            RunState = 4
	RunStateCompleted            RunState = 5
	RunStateDataDeleted          RunState = 6
	RunStateRequestedDataDeleted RunState = 7
	runStateSentinel             RunState = 8
)

func (rs RunState) String() string {
	switch rs {
	case RunStateUnknown:
		return "Unknown"
	case RunStateInitiated:
		return "Initiated"
	case RunStateRunning:
		return "Running"
	case RunStatePaused:
		return "Paused"
	case RunStateCancelled:
		return "Cancelled"
	case RunStateCompleted:
		return "Completed"
	case RunStateDataDeleted:
		return "DataDeleted"
	case RunStateRequestedDataDeleted:
		return "RequestedDataDeleted"
	default:
		return fmt.Sprintf("RunState(%d)", rs)
	}
}

func (rs RunState) Valid() bool {
	return rs > RunStateUnknown && rs < runStateSentinel
}

func (rs RunState) Finished() bool {
	switch rs {
	case RunStateCompleted, RunStateCancelled, RunStateRequestedDataDeleted, RunStateDataDeleted:
		return true
	default:
		return false
	}
}

// Stopped is the type of status that requires consumers to ignore the workflow record as it is in a stopped state. Only
// paused workflow records can be resumed and must be done so via the workflow API or the Run methods. All cancelled
// workflow records are cancelled permanently and cannot be undone whereas Pausing can be resumed.
func (rs RunState) Stopped() bool {
	switch rs {
	case RunStatePaused, RunStateCancelled, RunStateRequestedDataDeleted, RunStateDataDeleted:
		return true
	default:
		return false
	}
}

// RunStateController allows the interaction with a specific workflow record.
type RunStateController interface {
	// Pause will take the workflow record specified and move it into a temporary state where it will no longer be processed.
	// A paused workflow record can be resumed by calling Resume. ErrUnableToPause is returned when a workflow is not in a
	// state to be paused.
	Pause(ctx context.Context) error
	// Cancel can be called after Pause has been called. A paused run of the workflow can be indefinitely cancelled.
	// Once cancelled, DeleteData can be called and will move the run into an indefinite state of DataDeleted.
	// ErrUnableToCancel is returned when the workflow record is not in a state to be cancelled.
	Cancel(ctx context.Context) error
	// Resume can be called on a workflow record that has been paused. ErrUnableToResume is returned when the workflow
	// run is not in a state to be resumed.
	Resume(ctx context.Context) error
	// DeleteData can be called after a workflow record has been completed or cancelled. DeleteData should be used to
	// comply with the right to be forgotten such as complying with GDPR. ErrUnableToDelete is returned when the
	// workflow record is not in a state to be deleted.
	DeleteData(ctx context.Context) error
}

func NewRunStateController(store storeFunc, wr *WireRecord) RunStateController {
	return &runStateControllerImpl{
		record: wr,
		store:  store,
	}
}

type customDelete func(wr *WireRecord) ([]byte, error)

type runStateControllerImpl struct {
	record *WireRecord
	store  storeFunc
}

func (rsc *runStateControllerImpl) Pause(ctx context.Context) error {
	return rsc.update(ctx, RunStatePaused, ErrUnableToPause)
}

func (rsc *runStateControllerImpl) Resume(ctx context.Context) error {
	return rsc.update(ctx, RunStateRunning, ErrUnableToResume)
}

func (rsc *runStateControllerImpl) Cancel(ctx context.Context) error {
	return rsc.update(ctx, RunStateCancelled, ErrUnableToCancel)
}

func (rsc *runStateControllerImpl) DeleteData(ctx context.Context) error {
	return rsc.update(ctx, RunStateRequestedDataDeleted, ErrUnableToDelete)
}

func (rsc *runStateControllerImpl) update(ctx context.Context, rs RunState, invalidTransitionErr error) error {
	err := validateRunStateTransition(rsc.record, rs, invalidTransitionErr)
	if err != nil {
		return err
	}

	previousRunState := rsc.record.RunState
	rsc.record.RunState = rs
	return updateWireRecord(ctx, rsc.store, rsc.record, previousRunState)
}

func validateRunStateTransition(record *WireRecord, runState RunState, sentinelErr error) error {
	valid, ok := runStateTransitions[record.RunState]
	if !ok {
		return errors.Wrap(sentinelErr, "current run state is terminal", j.MKV{
			"record_id":           record.ID,
			"workflow_name":       record.WorkflowName,
			"run_state":           record.RunState.String(),
			"run_state_int_value": int(record.RunState),
		})
	}

	if !valid[runState] {
		msg := fmt.Sprintf("current run state cannot transition to %v", runState.String())
		return errors.Wrap(sentinelErr, msg, j.MKV{
			"record_id":           record.ID,
			"workflow_name":       record.WorkflowName,
			"run_state":           record.RunState.String(),
			"run_state_int_value": int(record.RunState),
		})
	}

	return nil
}

var runStateTransitions = map[RunState]map[RunState]bool{
	RunStateInitiated: {
		RunStateRunning: true,
		RunStatePaused:  true,
	},
	RunStateRunning: {
		RunStateCompleted: true,
		RunStatePaused:    true,
		RunStateCancelled: true,
	},
	RunStatePaused: {
		RunStateRunning:   true,
		RunStateCancelled: true,
	},
	RunStateCompleted: {
		RunStateRequestedDataDeleted: true,
	},
	RunStateCancelled: {
		RunStateRequestedDataDeleted: true,
	},
	RunStateRequestedDataDeleted: {
		RunStateDataDeleted: true,
	},
	RunStateDataDeleted: {
		RunStateRequestedDataDeleted: true,
	},
}

type noopRunStateController struct{}

func (c *noopRunStateController) Pause(ctx context.Context) error {
	return nil
}

func (c *noopRunStateController) Cancel(ctx context.Context) error {
	return nil
}

func (c *noopRunStateController) Resume(ctx context.Context) error {
	return nil
}

func (c *noopRunStateController) DeleteData(ctx context.Context) error {
	return nil
}

var _ RunStateController = (*noopRunStateController)(nil)
