package workflow

import (
	"context"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/errorcounter"
	"github.com/luno/workflow/internal/metrics"
)

// maybePause will either return a nil error if it has failed to pause the record and should be retried. A non-nil
// error is returned when no faults have taken place and the corresponding bool returns true when the Run is paused
// and returns false when the Run was not paused.
func maybePause[Type any, Status StatusType](
	ctx context.Context,
	pauseAfterErrCount int,
	counter errorcounter.ErrorCounter,
	originalErr error,
	processName string,
	run *Run[Type, Status],
	logger *logger,
) (paused bool, err error) {
	// Only keep track of errors only if we need to
	if pauseAfterErrCount == 0 {
		return false, nil
	}

	count := counter.Add(originalErr, processName, run.RunID)
	if count < pauseAfterErrCount {
		return false, nil
	}

	_, err = run.Pause(ctx)
	if err != nil {
		return false, err
	}

	logger.maybeDebug(ctx, "paused record after exceeding allowed error count", map[string]string{
		"workflow_name": run.WorkflowName,
		"foreign_id":    run.ForeignID,
		"run_id":        run.RunID,
	})

	// Run paused - now clear the error counter.
	counter.Clear(originalErr, processName, run.RunID)
	return true, nil
}

type autoPauseRetryConfig struct {
	enabled bool
	// limit determines the number of records in one lookup cycle.
	limit int
	// pollingFrequency is the frequency of the lookup cycle that looks up paused records that have met
	// or exceeded the resumeAfter duration.
	pollingFrequency time.Duration
	// resumeAfter is the duration that the record should remain paused for.
	resumeAfter time.Duration
}

func defaultAutoPauseRetryConfig() autoPauseRetryConfig {
	return autoPauseRetryConfig{
		enabled:          true,
		limit:            10,
		pollingFrequency: time.Minute,
		resumeAfter:      time.Hour,
	}
}

func autoRetryPausedRecordsForever[Type any, Status StatusType](w *Workflow[Type, Status]) {
	role := makeRole(w.Name(), "paused-records-auto-retry")
	processName := role

	w.run(role, processName, func(ctx context.Context) error {
		for {
			err := retryPausedRecords(
				ctx,
				w.Name(),
				w.recordStore.List,
				w.recordStore.Store,
				w.clock,
				processName,
				w.autoPauseRetryConfig.limit,
				w.autoPauseRetryConfig.resumeAfter,
			)
			if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-w.clock.After(w.autoPauseRetryConfig.pollingFrequency): // Slow and constant drip feed of paused records back into running state.
				continue
			}
		}
	}, w.defaultOpts.errBackOff)
}

type listFunc func(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error)

func retryPausedRecords(
	ctx context.Context,
	workflowName string,
	list listFunc,
	store storeFunc,
	clock clock.Clock,
	processName string,
	limit int,
	retryInterval time.Duration,
) error {
	t0 := clock.Now()

	rs, err := list(ctx, workflowName, 0, limit, OrderTypeAscending, FilterByRunState(RunStatePaused))
	if err != nil {
		return err
	}

	threshold := clock.Now().Add(-retryInterval)
	for _, r := range rs {
		if r.UpdatedAt.After(threshold) {
			continue
		}

		controller := NewRunStateController(store, &r)
		err := controller.Resume(ctx)
		if err != nil {
			return err
		}
	}

	metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t0).Seconds())
	return nil
}
