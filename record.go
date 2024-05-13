package workflow

import (
	"context"
	"testing"
	"time"
)

type Record[Type any, Status StatusType] struct {
	WireRecord
	Status Status
	Object *Type

	// stopper provides controls over the run state of the record. Record is not serializable and is not
	// intended to be and thus WireRecord exists as a serializable representation of a record.
	controller RunStateController
}

// Pause is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Pause(ctx)" to pause a record from inside a workflow which results in the record
// being temporarily left alone and will not be processed until it is resumed.
func (r *Record[Type, Status]) Pause(ctx context.Context) (Status, error) {
	err := r.controller.Pause(ctx)
	if err != nil {
		return 0, err
	}

	return Status(SkipTypeRunStateUpdate), nil
}

// Cancel is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Cancel(ctx)" to cancel a record from inside a workflow which results in the record
// being permanently left alone and will not be processed.
func (r *Record[Type, Status]) Cancel(ctx context.Context) (Status, error) {
	err := r.controller.Cancel(ctx)
	if err != nil {
		return 0, err
	}

	return Status(SkipTypeRunStateUpdate), nil
}

func NewTestingRecord[Type any, Status StatusType](t *testing.T, wr WireRecord, object Type) Record[Type, Status] {
	return Record[Type, Status]{
		WireRecord: wr,
		Status:     Status(wr.Status),
		Object:     &object,
		controller: &noopRunStateController{},
	}
}

type WireRecord struct {
	ID           int64
	WorkflowName string
	ForeignID    string
	RunID        string
	RunState     RunState
	Status       int
	Object       []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func buildConsumableRecord[Type any, Status StatusType](ctx context.Context, store storeFunc, wr *WireRecord, cd customDelete) (*Record[Type, Status], error) {
	var t Type
	err := Unmarshal(wr.Object, &t)
	if err != nil {
		return nil, err
	}

	controller := newRunStateController(wr, store, cd)
	record := Record[Type, Status]{
		WireRecord: *wr,
		Status:     Status(wr.Status),
		Object:     &t,
		controller: controller,
	}

	// The first time the record is consumed, it needs to be marked as RunStateRunning to represent that the record
	// has begun being processed. Even if the consumer errors then this should update should remain in place and
	// not be executed on the subsequent retries.
	if record.RunState == RunStateInitiated {
		err := controller.markAsRunning(ctx)
		if err != nil {
			return nil, err
		}

		record.RunState = RunStateRunning
	}

	return &record, nil
}
