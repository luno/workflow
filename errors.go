package workflow

import "errors"

var (
	ErrRecordNotFound       = errors.New("record not found")
	ErrTimeoutNotFound      = errors.New("timeout not found")
	ErrWorkflowInProgress   = errors.New("current workflow still in progress - retry once complete")
	ErrOutboxRecordNotFound = errors.New("outbox record not found")
	ErrInvalidTransition    = errors.New("invalid transition")
)

// ErrorCounter defines an interface for counting occurrences of errors keyed by stable labels.
// At least one label is required — labels should identify the process and run (e.g. processName, runID).
// The error value is not used for keying because error messages often contain dynamic data.
type ErrorCounter interface {
	Add(err error, label string, extras ...string) int
	Count(err error, label string, extras ...string) int
	Clear(err error, label string, extras ...string)
}
