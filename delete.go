package workflow

import (
	"context"
)

func deleteConsumer[Type any, Status StatusType](w *Workflow[Type, Status]) {
	role := makeRole(
		w.Name(),
		"delete",
		"consumer",
	)

	processName := makeRole("delete", "consumer")
	w.run(role, processName, func(ctx context.Context) error {
		topic := DeleteTopic(w.Name())
		stream, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(w.defaultOpts.pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer stream.Close()

		return Consume(
			ctx,
			w.Name(),
			processName,
			stream,
			runDelete(
				w.recordStore.Store,
				w.recordStore.Lookup,
				w.customDelete,
			),
			w.clock,
			0,
			w.defaultOpts.lagAlert,
		)
	}, w.defaultOpts.errBackOff)
}

func runDelete(
	store storeFunc,
	lookup lookupFunc,
	customDeleteFn customDelete,
) func(ctx context.Context, e *Event) error {
	return func(ctx context.Context, e *Event) error {
		record, err := lookup(ctx, e.ForeignID)
		if err != nil {
			return err
		}

		replacementData := []byte("{'result': 'deleted'}")
		// If a custom delete has been configured then use the custom delete
		if customDeleteFn != nil {
			bytes, err := customDeleteFn(record)
			if err != nil {
				return err
			}

			replacementData = bytes
		}

		record.Object = replacementData
		record.RunState = RunStateDataDeleted
		return updateRecord(
			ctx,
			store,
			record,
			RunStateRequestedDataDeleted,
		)
	}
}
