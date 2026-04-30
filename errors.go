package workflow

import "errors"

var (
	ErrRecordNotFound       = errors.New("record not found")
	ErrTimeoutNotFound      = errors.New("timeout not found")
	ErrWorkflowInProgress   = errors.New("current workflow still in progress - retry once complete")
	ErrOutboxRecordNotFound = errors.New("outbox record not found")
	ErrInvalidTransition    = errors.New("invalid transition")
)

// ErrorCounter defines an interface for counting errors keyed by stable labels.
// At least one label is required — labels should identify the process and run (e.g. processName, runID).
type ErrorCounter interface {
	Add(label string, extras ...string) int
	Count(label string, extras ...string) int
	Clear(label string, extras ...string)
}
