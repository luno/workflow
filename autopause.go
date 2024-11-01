package workflow

import (
	"context"

	"github.com/luno/workflow/internal/errorcounter"
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
	if count <= pauseAfterErrCount {
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
