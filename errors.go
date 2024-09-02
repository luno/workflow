package workflow

import (
	"github.com/luno/workflow/internal/errors"
)

var (
	ErrRecordNotFound              = errors.New("record not found")
	ErrTimeoutNotFound             = errors.New("timeout not found")
	ErrWorkflowInProgress          = errors.New("current workflow still in progress - retry once complete")
	ErrWorkflowNotRunning          = errors.New("trigger failed - workflow is not running")
	ErrStatusProvidedNotConfigured = errors.New("status provided is not configured for workflow")
	ErrOutboxRecordNotFound        = errors.New("outbox record not found")
	ErrUnableToPause               = errors.New("run is unable to be paused")
	ErrUnableToResume              = errors.New("run is unable to be resumed")
	ErrUnableToCancel              = errors.New("run is unable to be cancelled")
	ErrUnableToDelete              = errors.New("cannot delete data as run has not finished")
)
