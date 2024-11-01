package workflow

import (
	"context"
	"fmt"
	"strconv"
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
		return "Data Deleted"
	case RunStateRequestedDataDeleted:
		return "Requested Data Deleted"
	default:
		return "RunState(" + strconv.FormatInt(int64(rs), 10) + ")"
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

// Stopped is the type of status that requires consumers to ignore the workflow run as it is in a stopped state. Only
// paused workflow runs can be resumed and must be done so via the workflow API or the Run methods. All cancelled
// workflow runs are cancelled permanently and cannot be undone whereas Pausing can be resumed.
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
	// Pause will take the workflow run specified and move it into a temporary state where it will no longer be processed.
	// A paused workflow run can be resumed by calling Resume. ErrUnableToPause is returned when a workflow is not in a
	// state to be paused.
	Pause(ctx context.Context) error
	// Cancel can be called after Pause has been called. A paused run of the workflow can be indefinitely cancelled.
	// Once cancelled, DeleteData can be called and will move the run into an indefinite state of DataDeleted.
	// ErrUnableToCancel is returned when the workflow record is not in a state to be cancelled.
	Cancel(ctx context.Context) error
	// Resume can be called on a workflow run that has been paused. ErrUnableToResume is returned when the workflow
	// run is not in a state to be resumed.
	Resume(ctx context.Context) error
	// DeleteData can be called after a workflow run has been completed or cancelled. DeleteData should be used to
	// comply with the right to be forgotten such as complying with GDPR. ErrUnableToDelete is returned when the
	// workflow run is not in a state to be deleted.
	DeleteData(ctx context.Context) error
}

func NewRunStateController(store storeFunc, wr *Record) RunStateController {
	return &runStateControllerImpl{
		record: wr,
		store:  store,
	}
}

type customDelete func(wr *Record) ([]byte, error)

type runStateControllerImpl struct {
	record *Record
	store  storeFunc
}

func (rsc *runStateControllerImpl) Pause(ctx context.Context) error {
	return rsc.update(ctx, RunStatePaused)
}

func (rsc *runStateControllerImpl) Resume(ctx context.Context) error {
	return rsc.update(ctx, RunStateRunning)
}

func (rsc *runStateControllerImpl) Cancel(ctx context.Context) error {
	return rsc.update(ctx, RunStateCancelled)
}

func (rsc *runStateControllerImpl) DeleteData(ctx context.Context) error {
	return rsc.update(ctx, RunStateRequestedDataDeleted)
}

func (rsc *runStateControllerImpl) update(ctx context.Context, rs RunState) error {
	valid, ok := runStateTransitions[rsc.record.RunState]
	if !ok || !valid[rs] {
		return fmt.Errorf("invalid RunState: from %s | to %s", rsc.record.RunState, rs)
	}

	previousRunState := rsc.record.RunState
	rsc.record.RunState = rs
	return updateRecord(ctx, rsc.store, rsc.record, previousRunState)
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
