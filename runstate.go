package workflow

import (
	"context"
	"fmt"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

type RunState int

const (
	RunStateUnknown     RunState = 0
	RunStateInitiated   RunState = 1
	RunStateRunning     RunState = 2
	RunStatePaused      RunState = 3
	RunStateCancelled   RunState = 4
	RunStateCompleted   RunState = 5
	RunStateDataDeleted RunState = 6
	runStateSentinel    RunState = 7
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
	default:
		return fmt.Sprintf("RunState(%d)", rs)
	}
}

func (rs RunState) Valid() bool {
	return RunStateUnknown > rs && rs < runStateSentinel
}

func (rs RunState) Finished() bool {
	switch rs {
	case RunStateCompleted, RunStateCancelled, RunStateDataDeleted:
		return true
	default:
		return false
	}
}

// Stopped is the type of status that requires consumers to ignore the workflow record as it is in a stopped state. Only
// paused workflow records can be resumed and must be done so via the workflow API. All cancelled workflow records must first be
// paused and then cancelled.
func (rs RunState) Stopped() bool {
	switch rs {
	case RunStatePaused, RunStateCancelled, RunStateDataDeleted:
		return true
	default:
		return false
	}
}

// RunStateController allows the interaction with a specific workflow record.
type RunStateController[Status StatusType] interface {
	stopper[Status]
	// Resume can be called on a workflow record that has been paused. ErrUnableToResume is returned when the workflow
	// run is not in a state to be resumed.
	Resume(ctx context.Context) (Status, error)
	// DeleteData can be called after a workflow record has been completed or cancelled. DeleteData should be used to
	// comply with the right to be forgotten such as complying with GDPR. ErrUnableToDelete is returned when the
	// workflow record is not in a state to be deleted.
	DeleteData(ctx context.Context) (Status, error)

	// markAsRunning is an internal controller that is used to update RunStateInitiated records to RunStateRunning
	markAsRunning(ctx context.Context) (Status, error)
}

// stopper defines the methods that either are a temporary stopping of processing or a permanent stopping such as
// cancellation.
type stopper[Status StatusType] interface {
	// Pause will take the workflow record specified and move it into a temporary state where it will no longer be processed.
	// A paused workflow record can be resumed by calling Resume. ErrUnableToPause is returned when a workflow is not in a
	// state to be paused.
	Pause(ctx context.Context) (Status, error)

	// Cancel can be called after Pause has been called. A paused run of the workflow can be indefinitely cancelled.
	// Once cancelled, DeleteData can be called and will move the run into an indefinite state of DataDeleted.
	// ErrUnableToCancel is returned when the workflow record is not in a state to be cancelled.
	Cancel(ctx context.Context) (Status, error)
}

func newRunStateController[Status StatusType](wr *WireRecord, rs RecordStore, storeFunc storeAndEmitFunc) RunStateController[Status] {
	return &runStateControllerImpl[Status]{
		record:          wr,
		recordStore:     rs,
		storeAndEmitter: storeFunc,
	}
}

type runStateControllerImpl[Status StatusType] struct {
	record          *WireRecord
	customDelete    func() ([]byte, error)
	recordStore     RecordStore
	storeAndEmitter storeAndEmitFunc
}

func (rsc *runStateControllerImpl[Status]) markAsRunning(ctx context.Context) (Status, error) {
	err := validateRunStateTransition(rsc.record, RunStateRunning, ErrUnableToMarkAsRunning)
	if err != nil {
		return 0, err
	}

	currentRunState := rsc.record.RunState

	rsc.record.RunState = RunStateRunning
	return Status(SkipTypeRunStateUpdate), rsc.storeAndEmitter(ctx, rsc.recordStore, rsc.record, currentRunState)
}

func (rsc *runStateControllerImpl[Status]) Pause(ctx context.Context) (Status, error) {
	err := validateRunStateTransition(rsc.record, RunStatePaused, ErrUnableToPause)
	if err != nil {
		return 0, err
	}

	currentRunState := rsc.record.RunState

	rsc.record.RunState = RunStatePaused
	return Status(SkipTypeRunStateUpdate), rsc.storeAndEmitter(ctx, rsc.recordStore, rsc.record, currentRunState)
}

func (rsc *runStateControllerImpl[Status]) Resume(ctx context.Context) (Status, error) {
	err := validateRunStateTransition(rsc.record, RunStateRunning, ErrUnableToResume)
	if err != nil {
		return 0, err
	}

	currentRunState := rsc.record.RunState

	rsc.record.RunState = RunStateRunning
	return Status(SkipTypeRunStateUpdate), rsc.storeAndEmitter(ctx, rsc.recordStore, rsc.record, currentRunState)
}

func (rsc *runStateControllerImpl[Status]) Cancel(ctx context.Context) (Status, error) {
	err := validateRunStateTransition(rsc.record, RunStateCancelled, ErrUnableToCancel)
	if err != nil {
		return 0, err
	}

	currentRunState := rsc.record.RunState

	rsc.record.RunState = RunStateCancelled
	return Status(SkipTypeRunStateUpdate), rsc.storeAndEmitter(ctx, rsc.recordStore, rsc.record, currentRunState)
}

func (rsc *runStateControllerImpl[Status]) DeleteData(ctx context.Context) (Status, error) {
	err := validateRunStateTransition(rsc.record, RunStateDataDeleted, ErrUnableToDelete)
	if err != nil {
		return 0, err
	}

	replacementData := []byte("Deleted")

	// If a custom delete has been configured then use the custom delete
	if rsc.customDelete != nil {
		b, err := rsc.customDelete()
		if err != nil {
			return 0, err
		}

		replacementData = b
	}

	currentRunState := rsc.record.RunState

	rsc.record.RunState = RunStateDataDeleted
	rsc.record.Object = replacementData
	return Status(SkipTypeRunStateUpdate), rsc.storeAndEmitter(ctx, rsc.recordStore, rsc.record, currentRunState)
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
		RunStateDataDeleted: true,
	},
	RunStateCancelled: {
		RunStateDataDeleted: true,
	},
	RunStateDataDeleted: {
		RunStateDataDeleted: true,
	},
}

func (w *Workflow[Type, Status]) RunStateController(ctx context.Context, foreignID, runID string) (RunStateController[Status], error) {
	// TODO: Migrate to lookup by foreignID and runID
	r, err := w.recordStore.Latest(ctx, w.Name, foreignID)
	if err != nil {
		return nil, err
	}

	return newRunStateController[Status](r, w.recordStore, storeAndEmit), nil
}
