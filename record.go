package workflow

import (
	"context"
	"time"
)

type Record[Type any, Status StatusType] struct {
	WireRecord
	Status Status
	Object *Type

	// stopper provides controls over the run state of the record. Record is not serializable and is not
	// intended to be and thus WireRecord exists as a serializable representation of a record.
	stopper[Status]
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

func buildConsumableRecord[Type any, Status StatusType](ctx context.Context, store RecordStore, storeFunc storeAndEmitFunc, wr *WireRecord, cd customDelete) (*Record[Type, Status], error) {
	var t Type
	err := Unmarshal(wr.Object, &t)
	if err != nil {
		return nil, err
	}

	controller := newRunStateController[Status](wr, store, storeFunc, cd)
	record := Record[Type, Status]{
		WireRecord: *wr,
		Status:     Status(wr.Status),
		Object:     &t,
		stopper:    controller,
	}

	// The first time the record is consumed, it needs to be marked as RunStateRunning to represent that the record
	// has begun being processed. Even if the consumer errors then this should update should remain in place and
	// not be executed on the subsequent retries.
	if record.RunState == RunStateInitiated {
		_, err := controller.markAsRunning(ctx)
		if err != nil {
			return nil, err
		}

		record.RunState = RunStateRunning
	}

	return &record, nil
}
