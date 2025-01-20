package workflow

import (
	"context"
	"errors"
	"strconv"
	"time"

	"k8s.io/utils/clock"

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

	processName := role
	w.run(role, processName, func(ctx context.Context) error {
		topic := RunStateChangeTopic(w.Name())
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

		return runHooks(
			ctx,
			w.Name(),
			processName,
			consumerStream,
			runState,
			w.recordStore.Lookup,
			hook,
			w.defaultOpts.lagAlert,
			w.clock,
		)
	}, w.defaultOpts.errBackOff)
}

func runHooks[Type any, Status StatusType](
	ctx context.Context,
	workflowName string,
	processName string,
	c Consumer,
	runState RunState,
	lookup lookupFunc,
	hook RunStateChangeHookFunc[Type, Status],
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

		rs, err := strconv.ParseInt(e.Headers[HeaderRunState], 10, 64)
		if err != nil {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "invalid or missing HeaderRunState").
				Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		if RunState(rs) != runState {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "mismatch on current run state and target run state").
				Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

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

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(workflowName, processName, e.CreatedAt, lagAlert, clock)
		t2 := clock.Now()

		var t Type
		err = Unmarshal(record.Object, &t)
		if err != nil {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "unable to unmarshal object").Inc()
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		err = hook(ctx, &TypedRecord[Type, Status]{
			Record: *record,
			Status: Status(record.Status),
			Object: &t,
		})
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t2).Seconds())
		return ack()
	}
}
