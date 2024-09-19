package workflow

import (
	"context"
)

// Run is a representation of a workflow run. It incorporates all the fields from the Record as well as
// having defined types for the Status and Object fields along with access to the RunStateController which
// controls the state of the run aka "RunState".
type Run[Type any, Status StatusType] struct {
	Record
	Status Status
	Object *Type

	// stopper provides controls over the run state of the record. Run is not serializable and is not
	// intended to be and thus Record exists as a serializable representation of a record.
	controller RunStateController
}

// Pause is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Pause(ctx)" to pause a record from inside a workflow which results in the record
// being temporarily left alone and will not be processed until it is resumed.
func (r *Run[Type, Status]) Pause(ctx context.Context) (Status, error) {
	err := r.controller.Pause(ctx)
	if err != nil {
		return 0, err
	}

	return Status(SkipTypeRunStateUpdate), nil
}

// Skip is a util function to skip the update and move on to the next event (consumer) or execution (callback)
func (r *Run[Type, Status]) Skip() (Status, error) {
	return Status(SkipTypeDefault), nil
}

// Cancel is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Cancel(ctx)" to cancel a record from inside a workflow which results in the record
// being permanently left alone and will not be processed.
func (r *Run[Type, Status]) Cancel(ctx context.Context) (Status, error) {
	err := r.controller.Cancel(ctx)
	if err != nil {
		return 0, err
	}

	return Status(SkipTypeRunStateUpdate), nil
}

func buildRun[Type any, Status StatusType](store storeFunc, wr *Record) (*Run[Type, Status], error) {
	var t Type
	err := Unmarshal(wr.Object, &t)
	if err != nil {
		return nil, err
	}

	controller := NewRunStateController(store, wr)
	record := Run[Type, Status]{
		Record:     *wr,
		Status:     Status(wr.Status),
		Object:     &t,
		controller: controller,
	}

	// The first time the record is consumed, it needs to be marked as RunStateRunning to represent that the record
	// has begun being processed. Even if the consumer errors then this should update should remain in place and
	// not be executed on the subsequent retries.
	if record.RunState == RunStateInitiated {
		record.RunState = RunStateRunning
	}

	return &record, nil
}
