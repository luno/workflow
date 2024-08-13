package workflow

import "time"

type WireRecord struct {
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
