package workflow

import (
	"bytes"
	"context"
	"io"
)

type callback[Type any, Status StatusType] struct {
	CallbackFunc CallbackFunc[Type, Status]
}

type CallbackFunc[Type any, Status StatusType] func(ctx context.Context, r *Run[Type, Status], reader io.Reader) (Status, error)

func (w *Workflow[Type, Status]) Callback(
	ctx context.Context,
	foreignID string,
	status Status,
	payload io.Reader,
) error {
	updateFn := newUpdater[Type, Status](w.recordStore.Lookup, w.recordStore.Store, w.statusGraph, w.clock)

	for _, s := range w.callback[status] {
		err := processCallback(
			ctx,
			w,
			status,
			s.CallbackFunc,
			foreignID,
			payload,
			w.recordStore.Latest,
			w.recordStore.Store,
			updateFn,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

type latestLookup func(ctx context.Context, workflowName, foreignID string) (*Record, error)

func processCallback[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	currentStatus Status,
	fn CallbackFunc[Type, Status],
	foreignID string,
	payload io.Reader,
	latest latestLookup,
	store storeFunc,
	updater updater[Type, Status],
) error {
	wr, err := latest(ctx, w.Name(), foreignID)
	if err != nil {
		return err
	}

	if Status(wr.Status) != currentStatus {
		// Latest record shows that the current status is in a different State than expected so skip.
		return nil
	}

	run, err := buildRun[Type, Status](store, wr)
	if err != nil {
		return err
	}

	if payload == nil {
		// Ensure that an empty value implementation of io.Reader is passed in instead of nil to avoid panic and
		// rather allow an unmarshalling error.
		payload = bytes.NewReader([]byte{})
	}

	next, err := fn(ctx, run, payload)
	if err != nil {
		return err
	}

	if skipUpdate(next) {
		w.logger.Debug(ctx, "skipping update", map[string]string{
			"description":   skipUpdateDescription(next),
			"workflow_name": w.Name(),
			"foreign_id":    run.ForeignID,
			"run_id":        run.RunID,
			"run_state":     run.RunState.String(),
			"record_status": run.Status.String(),
		})

		return nil
	}

	return updater(ctx, currentStatus, next, run)
}
