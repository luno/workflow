package workflow

import (
	"context"
	"time"

	"k8s.io/utils/clock"
)

// maybePause will either return a nil error if it has failed to pause the record and should be retried. A non-nil
// error is returned when no faults have taken place and the corresponding bool returns true when the Run is paused
// maybePause decides whether to pause a Run after its accumulated errors reach a configured threshold and performs the pause when the threshold is met.
// If pauseAfterErrCount is zero the function does nothing. When the Run is paused the error counter for that error/process/run is cleared.
// Returns true if the Run was paused, false otherwise; returns a non-nil error only if attempting to pause the Run fails.
func maybePause[Type any, Status StatusType](
	ctx context.Context,
	pauseAfterErrCount int,
	counter ErrorCounter,
	originalErr error,
	processName string,
	run *Run[Type, Status],
	logger Logger,
) (paused bool, err error) {
	// Only keep track of errors only if we need to
	if pauseAfterErrCount == 0 {
		return false, nil
	}

	count := counter.Add(originalErr, processName, run.RunID)
	if count < pauseAfterErrCount {
		return false, nil
	}

	_, err = run.Pause(ctx, "max error retry threshold hit - automatically paused")
	if err != nil {
		return false, err
	}

	logger.Debug(ctx, "paused record after exceeding allowed error count", map[string]string{
		"workflow_name": run.WorkflowName,
		"foreign_id":    run.ForeignID,
		"run_id":        run.RunID,
	})

	// Run paused - now clear the error counter.
	counter.Clear(originalErr, processName, run.RunID)
	return true, nil
}

func pausedRecordsRetryConsumer[Type any, Status StatusType](w *Workflow[Type, Status]) {
	role := makeRole(
		w.Name(),
		"paused",
		"records",
		"retry",
		"consumer",
	)

	processName := makeRole(
		"paused",
		"records",
		"retry",
		"consumer",
	)
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

		lagAlert := w.pausedRecordsRetry.resumeAfter * 3
		if lagAlert < time.Minute {
			lagAlert = w.pausedRecordsRetry.resumeAfter + time.Minute*5
		}

		return consume(
			ctx,
			w.Name(),
			processName,
			stream,
			autoRetryConsumer(
				w.recordStore.Lookup,
				w.recordStore.Store,
				w.clock,
				w.pausedRecordsRetry.resumeAfter,
			),
			w.clock,
			w.pausedRecordsRetry.resumeAfter,
			lagAlert,
			filterByRunState(RunStatePaused),
		)
	}, w.defaultOpts.errBackOff)
}

func autoRetryConsumer(
	lookupFn lookupFunc,
	store storeFunc,
	clock clock.Clock,
	retryInterval time.Duration,
) func(ctx context.Context, e *Event) error {
	return func(ctx context.Context, e *Event) error {
		record, err := lookupFn(ctx, e.ForeignID)
		if err != nil {
			return err
		}

		if record.RunState != RunStatePaused {
			return nil
		}

		threshold := clock.Now().Add(-retryInterval)
		if record.UpdatedAt.After(threshold) {
			return nil
		}

		controller := NewRunStateController(store, record)
		err = controller.Resume(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}

type pausedRecordsRetry struct {
	enabled bool
	// resumeAfter is the duration that the record should remain paused for.
	resumeAfter time.Duration
}

func defaultPausedRecordsRetry() pausedRecordsRetry {
	resumeAfter := time.Hour
	return pausedRecordsRetry{
		enabled:     true,
		resumeAfter: resumeAfter,
	}
}