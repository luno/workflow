package workflow

import "time"

// Record is the cornerstone of Workflow. Record must always be wire compatible with no generics as it's intended
// purpose is to be the
type Record struct {
	ID           int64
	WorkflowName string
	ForeignID    string
	RunID        string
	RunState     RunState
	Status       int
	Object       []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TypedRecord differs from Record in that it contains a Typed Object and Typed Status
type TypedRecord[Type any, Status StatusType] struct {
	Record
	Status Status
	Object *Type
}
