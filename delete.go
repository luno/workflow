package workflow

import (
	"context"
	"errors"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
)

func deleteConsumer[Type any, Status StatusType](w *Workflow[Type, Status]) {
	role := makeRole(
		w.Name(),
		"delete",
		"consumer",
	)

	processName := role
	w.run(role, processName, func(ctx context.Context) error {
		topic := DeleteTopic(w.Name())
		consumerStream, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(w.defaultOpts.pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer consumerStream.Close()

		return DeleteForever(
			ctx,
			w.Name(),
			processName,
			consumerStream,
			w.recordStore.Store,
			w.recordStore.Lookup,
			w.customDelete,
			w.defaultOpts.lagAlert,
			w.clock,
		)
	}, w.defaultOpts.errBackOff)
}

func DeleteForever(
	ctx context.Context,
	workflowName string,
	processName string,
	c Consumer,
	store storeFunc,
	lookup lookupFunc,
	customDeleteFn customDelete,
	lagAlert time.Duration,
	clock clock.Clock,
) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e, ack, err := c.Recv(ctx)
		if err != nil {
			return err
		}

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(workflowName, processName, e.CreatedAt, lagAlert, clock)

		record, err := lookup(ctx, e.ForeignID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "record not found").Inc()
				err = ack()
				if err != nil {
					return err
				}

				continue
			} else {
				return err
			}
		}

		t2 := clock.Now()
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
		err = updateRecord(ctx, store, record, RunStateRequestedDataDeleted)
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t2).Seconds())
		return ack()
	}
}
