package workflow

import "errors"

var (
	ErrRecordNotFound       = errors.New("record not found")
	ErrTimeoutNotFound      = errors.New("timeout not found")
	ErrWorkflowInProgress   = errors.New("current workflow still in progress - retry once complete")
	ErrOutboxRecordNotFound = errors.New("outbox record not found")
	ErrInvalidTransition    = errors.New("invalid transition")
)
