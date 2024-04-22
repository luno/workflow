package workflow

import (
	"context"
	"time"

	"github.com/luno/jettison/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/luno/workflow/workflowpb"
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

func (r *WireRecord) ProtoMarshal() ([]byte, error) {
	pb, err := proto.Marshal(ToProto(r))
	if err != nil {
		return nil, errors.Wrap(err, "failed to proto marshal record")
	}

	return pb, nil
}

func ToProto(r *WireRecord) *workflowpb.Record {
	return &workflowpb.Record{
		WorkflowName: r.WorkflowName,
		ForeignId:    r.ForeignID,
		RunId:        r.RunID,
		Status:       int32(r.Status),
		RunState:     int32(r.RunState),
		Object:       r.Object,
		CreatedAt:    timestamppb.New(r.CreatedAt),
		UpdatedAt:    timestamppb.New(r.UpdatedAt),
	}
}

func UnmarshalRecord(b []byte) (*WireRecord, error) {
	var wpb workflowpb.Record
	err := proto.Unmarshal(b, &wpb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to proto marshal record")
	}

	return &WireRecord{
		WorkflowName: wpb.WorkflowName,
		ForeignID:    wpb.ForeignId,
		RunID:        wpb.RunId,
		Status:       int(wpb.Status),
		RunState:     RunState(wpb.RunState),
		Object:       wpb.Object,
		CreatedAt:    wpb.CreatedAt.AsTime(),
		UpdatedAt:    wpb.UpdatedAt.AsTime(),
	}, nil
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
