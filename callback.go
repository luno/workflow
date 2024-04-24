package workflow

import (
	"bytes"
	"context"
	"io"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/log"
)

type callback[Type any, Status StatusType] struct {
	CallbackFunc CallbackFunc[Type, Status]
}

type CallbackFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], reader io.Reader) (Status, error)

func (w *Workflow[Type, Status]) Callback(ctx context.Context, foreignID string, status Status, payload io.Reader) error {
	for _, s := range w.callback[status] {
		err := processCallback(ctx, w, status, s.CallbackFunc, foreignID, payload, w.recordStore.Latest, safeUpdate, storeAndEmit)
		if err != nil {
			return err
		}
	}

	return nil
}

type latestLookup func(ctx context.Context, workflowName, foreignID string) (*WireRecord, error)

func processCallback[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	currentStatus Status,
	fn CallbackFunc[Type, Status],
	foreignID string,
	payload io.Reader,
	latest latestLookup,
	updater safeUpdater,
	storeAndEmitter storeAndEmitFunc,
) error {
	wr, err := latest(ctx, w.Name, foreignID)
	if err != nil {
		return errors.Wrap(err, "failed to latest record for callback", j.MKV{
			"foreign_id": foreignID,
		})
	}

	if Status(wr.Status) != currentStatus {
		// Latest record shows that the current status is in a different State than expected so skip.
		return nil
	}

	record, err := buildConsumableRecord[Type, Status](ctx, w.recordStore, storeAndEmitter, wr, w.customDelete)
	if err != nil {
		return err
	}

	if payload == nil {
		// Ensure that an empty value implementation of io.Reader is passed in instead of nil to avoid panic and
		// rather allow an unmarshalling error.
		payload = bytes.NewReader([]byte{})
	}

	next, err := fn(ctx, record, payload)
	if err != nil {
		return err
	}

	if skipUpdate(next) {
		if w.debugMode {
			log.Info(ctx, "skipping update", j.MKV{
				"description":   skipUpdateDescription(next),
				"record_id":     record.ID,
				"workflow_name": w.Name,
				"foreign_id":    record.ForeignID,
				"run_id":        record.RunID,
				"run_state":     record.RunState.String(),
				"record_status": record.Status.String(),
			})
		}
		return nil
	}

	object, err := Marshal(&record.Object)
	if err != nil {
		return err
	}

	runState := RunStateRunning
	isEnd := w.endPoints[next]
	if isEnd {
		runState = RunStateCompleted
	}

	update := &WireRecord{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     runState,
		Status:       int(next),
		Object:       object,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    w.clock.Now(),
	}

	return updater(ctx, w.recordStore, w.graph, int(currentStatus), update)
}
