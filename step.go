package workflow

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/luno/workflow/internal/errorcounter"
	"github.com/luno/workflow/internal/metrics"
)

func consumeStepEvents[Type any, Status StatusType](
	w *Workflow[Type, Status],
	currentStatus Status,
	p consumerConfig[Type, Status],
	shard, totalShards int,
) {
	role := makeRole(
		w.Name(),
		strconv.FormatInt(int64(currentStatus), 10),
		"consumer",
		strconv.FormatInt(int64(shard), 10),
		"of",
		strconv.FormatInt(int64(totalShards), 10),
	)

	// processName can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeoutstore, or offset store.
	processName := makeRole(
		currentStatus.String(),
		"consumer",
		strconv.FormatInt(int64(shard), 10),
		"of",
		strconv.FormatInt(int64(totalShards), 10),
	)

	topic := Topic(w.Name(), int(currentStatus))

	errBackOff := w.defaultOpts.errBackOff
	if p.errBackOff > 0 {
		errBackOff = p.errBackOff
	}

	pollingFrequency := w.defaultOpts.pollingFrequency
	if p.pollingFrequency > 0 {
		pollingFrequency = p.pollingFrequency
	}

	lagAlert := w.defaultOpts.lagAlert
	if p.lagAlert > 0 {
		lagAlert = p.lagAlert
	}

	pauseAfterErrCount := w.defaultOpts.pauseAfterErrCount
	if p.pauseAfterErrCount != 0 {
		pauseAfterErrCount = p.pauseAfterErrCount
	}

	lag := w.defaultOpts.lag
	if p.lag > 0 {
		lag = p.lag
	}

	w.run(role, processName, func(ctx context.Context) error {
		stream, err := w.eventStreamer.NewConsumer(
			ctx,
			topic,
			role,
			WithConsumerPollFrequency(pollingFrequency),
		)
		if err != nil {
			return err
		}
		defer stream.Close()

		updater := newUpdater[Type, Status](w.recordStore.Lookup, w.recordStore.Store, w.statusGraph, w.clock)
		return consume(
			ctx,
			w.Name(),
			processName,
			stream,
			stepConsumer(
				w.Name(),
				processName,
				p.consumer,
				currentStatus,
				w.recordStore.Lookup,
				w.recordStore.Store,
				w.logger,
				updater,
				pauseAfterErrCount,
				w.errorCounter,
			),
			w.clock,
			lag,
			lagAlert,
			shardFilter(shard, totalShards),
		)
	}, errBackOff)
}

func stepConsumer[Type any, Status StatusType](
	workflowName string,
	processName string,
	stepLogic ConsumerFunc[Type, Status],
	currentStatus Status,
	lookupFn lookupFunc,
	store storeFunc,
	logger Logger,
	updater updater[Type, Status],
	pauseAfterErrCount int,
	errorCounter errorcounter.ErrorCounter,
) func(ctx context.Context, e *Event) error {
	return func(ctx context.Context, e *Event) error {
		record, err := lookupFn(ctx, e.ForeignID)
		if errors.Is(err, ErrRecordNotFound) {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "record not found").Inc()
			return nil
		} else if err != nil {
			return err
		}

		// Check to see if record is in expected state. If the status isn't in the expected state then skip for
		// idempotency.
		if record.Status != int(currentStatus) {
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "record status not in expected state").
				Inc()
			return nil
		}

		if record.RunState.Stopped() {
			logger.Debug(ctx, "Skipping consumption of stopped workflow record", map[string]string{
				"event_id":       strconv.FormatInt(e.ID, 10),
				"workflow":       record.WorkflowName,
				"run_id":         record.RunID,
				"foreign_id":     record.ForeignID,
				"process_name":   processName,
				"current_status": strconv.FormatInt(int64(record.Status), 10),
				"run_state":      record.RunState.String(),
			})
			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "record stopped").Inc()
			return nil
		}

		run, err := buildRun[Type, Status](store, record)
		if err != nil {
			return err
		}

		next, err := stepLogic(ctx, run)
		if err != nil {
			originalErr := err
			paused, err := maybePause(ctx, pauseAfterErrCount, errorCounter, originalErr, processName, run, logger)
			if err != nil {
				return fmt.Errorf("pause error: %v, meta: %v", err, map[string]string{
					"run_id":     record.RunID,
					"foreign_id": record.ForeignID,
				})
			}

			if paused {
				// Move onto the next event as a record has been paused and a new event is emitted
				// when it is resumed.
				return nil
			}

			// The record was not paused and the original error is not nil. Pass back up for retrying.
			return fmt.Errorf("consumer error: %v, meta: %v", originalErr, map[string]string{
				"run_id":     record.RunID,
				"foreign_id": record.ForeignID,
			})
		}

		if skipUpdate(next) {
			logger.Debug(ctx, "skipping update", map[string]string{
				"description":   skipUpdateDescription(next),
				"workflow_name": workflowName,
				"foreign_id":    run.ForeignID,
				"run_id":        run.RunID,
				"run_state":     run.RunState.String(),
				"record_status": run.Status.String(),
			})

			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "next value specified skip").Inc()
			return nil
		}

		return updater(ctx, Status(record.Status), next, run)
	}
}

func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}

	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
