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
	ErrOutboxRecordNotFound        = errors.New("outbox record not found", j.C("ERR_1ef1afdf9f7ae684"))
	ErrUnableToPause               = errors.New("run is unable to be paused", j.C("ERR_3b776661fe2c56c7"))
	ErrUnableToResume              = errors.New("run is unable to be resumed", j.C("ERR_fdbedb1059368f3e"))
	ErrUnableToCancel              = errors.New("run is unable to be cancelled", j.C("ERR_9128169c3d47eb1d"))
	ErrUnableToDelete              = errors.New("cannot delete data as run has not finished", j.C("ERR_2dec819246977dd9"))
	ErrUnableToMarkAsRunning       = errors.New("run is unable to be marked as Running", j.C("ERR_704b88a1eddad3dc"))
)

type Error interface {
	error
}
