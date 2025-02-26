package workflow

import (
	"context"

	"github.com/luno/workflow/internal/metrics"
)

// RunStateChangeHookFunc defines the function signature for all hooks associated to the run.
type RunStateChangeHookFunc[Type any, Status StatusType] func(ctx context.Context, record *TypedRecord[Type, Status]) error

func runStateChangeHookConsumer[Type any, Status StatusType](
	w *Workflow[Type, Status],
	runState RunState,
	hook RunStateChangeHookFunc[Type, Status],
) {
	role := makeRole(
		w.Name(),
		runState.String(),
		"run-state-change-hook",
		"consumer",
	)

	processName := makeRole(runState.String(), "run-state-change-hook", "consumer")
	w.run(role, processName, func(ctx context.Context) error {
		topic := RunStateChangeTopic(w.Name())
		stream, err := w.eventStreamer.NewReceiver(
			ctx,
			topic,
			role,
			WithReceiverPollFrequency(w.defaultOpts.pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer stream.Close()

		return consume(
			ctx,
			w.Name(),
			processName,
			stream,
			runHook(
				w.Name(),
				processName,
				w.recordStore.Lookup,
				hook,
			),
			w.clock,
			0,
			w.defaultOpts.lagAlert,
			filterByRunState(runState),
		)
	}, w.defaultOpts.errBackOff)
}

func runHook[Type any, Status StatusType](
	workflowName string,
	processName string,
	lookup lookupFunc,
	hook RunStateChangeHookFunc[Type, Status],
) func(ctx context.Context, e *Event) error {
	return func(ctx context.Context, e *Event) error {
		record, err := lookup(ctx, e.ForeignID)
		if err != nil {
			return err
		}

		var t Type
		err = Unmarshal(record.Object, &t)
		if err != nil {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "unable to unmarshal object").Inc()
			return nil
		}

		return hook(ctx, &TypedRecord[Type, Status]{
			Record: *record,
			Status: Status(record.Status),
			Object: &t,
		})
	}
}
