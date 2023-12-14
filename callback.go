package workflow

import (
	"bytes"
	"context"
	"io"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

type callback[Type any, Status StatusType] struct {
	DestinationStatus Status
	CallbackFunc      CallbackFunc[Type, Status]
}

type CallbackFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], reader io.Reader) (bool, error)

func (w *Workflow[Type, Status]) Callback(ctx context.Context, foreignID string, status Status, payload io.Reader) error {
	for _, s := range w.callback[status] {
		err := processCallback(ctx, w, status, s.DestinationStatus, s.CallbackFunc, foreignID, payload)
		if err != nil {
			return err
		}
	}

	return nil
}

func processCallback[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], currentStatus, destinationStatus Status, fn CallbackFunc[Type, Status], foreignID string, payload io.Reader) error {
	latest, err := w.recordStore.Latest(ctx, w.Name, foreignID)
	if err != nil {
		return errors.Wrap(err, "failed to latest record for callback", j.MKV{
			"foreign_id": foreignID,
		})
	}

	if Status(latest.Status) != currentStatus {
		// Latest record shows that the current status is in a different State than expected so skip.
		return nil
	}

	var t Type
	err = Unmarshal(latest.Object, &t)
	if err != nil {
		return err
	}

	record := &Record[Type, Status]{
		WireRecord: *latest,
		Object:     &t,
	}

	if payload == nil {
		// Ensure that an empty value implementation of io.Reader is passed in instead of nil to avoid panic and
		// rather allow an unmarshalling error.
		payload = bytes.NewReader([]byte{})
	}

	ok, err := fn(ctx, record, payload)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	object, err := Marshal(&record.Object)
	if err != nil {
		return err
	}

	wr := &WireRecord{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		Status:       int(destinationStatus),
		IsStart:      false,
		IsEnd:        w.endPoints[destinationStatus],
		Object:       object,
		CreatedAt:    record.CreatedAt,
	}

	return update(ctx, w.eventStreamerFn, w.recordStore, wr)
}
