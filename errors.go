package workflow

import "github.com/luno/jettison/errors"

var (
	ErrWorkflowShutdown            = errors.New("workflow has been shutdown")
	ErrCursorNotFound              = errors.New("cursor not found")
	ErrRecordNotFound              = errors.New("record not found")
	ErrTimeoutNotFound             = errors.New("timeout not found")
	ErrRunIDNotFound               = errors.New("run ID not found")
	ErrWorkflowInProgress          = errors.New("current workflow still in progress - retry once complete")
	ErrWorkflowNotRunning          = errors.New("trigger failed - workflow is not running")
	ErrStatusProvidedNotConfigured = errors.New("status provided is not configured for workflow")
)
