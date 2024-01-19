package workflow

import (
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

var (
	ErrRecordNotFound              = errors.New("record not found", j.C("ERR_6d982e73339f351a"))
	ErrTimeoutNotFound             = errors.New("timeout not found", j.C("ERR_2f6c8edf63f7ac8d"))
	ErrWorkflowInProgress          = errors.New("current workflow still in progress - retry once complete", j.C("ERR_cd79765555450db7"))
	ErrWorkflowNotRunning          = errors.New("trigger failed - workflow is not running", j.C("ERR_6b414d1eb843a681"))
	ErrStatusProvidedNotConfigured = errors.New("status provided is not configured for workflow", j.C("ERR_169c7465995cf7aa"))
)
