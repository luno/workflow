package workflow

import "time"

// Record is the cornerstone of Workflow. Record must always be wire compatible with no generics as it's intended
// purpose is to be the persisted data structure of a Run.
type Record struct {
	// WorkflowName is the name of the workflow associated with this record. It helps to identify which workflow
	// this record belongs to.
	WorkflowName string

	// ForeignID is an external identifier for this record, often used to associate it with an external system or
	// resource. This can be useful for linking data across systems.
	ForeignID string

	// RunID uniquely identifies the specific execution or instance of the workflow Run and is a UUID v4.
	// It allows tracking of different runs of the same workflow and foreign id.
	RunID string

	// RunState represents the current state of the workflow Run (e.g., running, paused, completed).
	// This field is important for understanding the lifecycle status of the run.
	RunState RunState

	// Status is an integer representing the numerical status code of the record.
	// Terminal, or end, statuses are written as past tense as they are fact and cannot change but present continuous
	// tense is used to describe the current process taking place (e.g. submitting, creating, storing)
	Status int

	// Object contains the actual data associated with the workflow Run, serialized as bytes.
	// This could be any kind of structured data related to the workflow execution. To unmarshal this data Unmarshal
	// must be used to do so and the type matches the generic type called "Type" provided to the workflow builder
	// ( e.g. NewBuilder[Type any, ...])
	Object []byte

	// CreatedAt is the timestamp when the record was created, providing context on when the workflow Run was initiated.
	CreatedAt time.Time

	// UpdatedAt is the timestamp when the record was last updated. This helps track the most recent changes made to
	// the record.
	UpdatedAt time.Time

	// Meta stores any additional metadata related to the record, such as human-readable reasons for the RunState
	// or other contextual information that can assist with debugging or auditing.
	Meta Meta
}

// Meta provides contextual information such as the string value of the Status at the given time of the last update
// and the reason the RunState is it's current value is uncommon or the need to know enables debugging. This is
// particularly useful when accessing the data of the record without the ability to cast it to a TypedRecord which is
// often the case when debugging the data via the RecordStore.
type Meta struct {
	// RunStateReason provides a human-readable explanation for the current run state.
	// This field helps to understand the cause behind the current state of the run, such as "Paused", "Canceled", or "Deleted".
	// For instance, calling functions like Pause, Cancel, or DeleteData will update this field with a descriptive reason
	// that explains why the run is in its current state, making it easier to understand the context behind the state transition.
	RunStateReason string

	// StatusDescription provides a human-readable version of the Status field (int).
	// It allows for better understanding and readability of the status at the time of the computation, especially when reviewing
	// historical data. While the `Status` field stores the status as an integer, `StatusDescription` maps that status to a
	// corresponding string description, making it easier to interpret the status without needing to refer to status codes.
	StatusDescription string

	// Version defines the version of the record. The Workflow increments this value before storing it in the
	// RecordStore and uses it for data validation when consuming an event. The event will contain a matching version,
	// ensuring that the event and the record from the RecordStore align, and that the data is not stale. This versioning
	// mechanism also helps address replication lag issues, particularly in systems using databases that experience
	// replication delays between replicas. By matching the version, Workflow ensures that the version collected from
	// the reader is indeed the version expected. If the version in the RecordStore is greater than the event can be
	// ignored as it has already been processed.
	Version uint

	// TraceOrigin contains a trace of the origin or source where the workflow Run was triggered from.
	// It provides a line trace or path that helps track where the execution was initiated,
	// offering context for debugging or auditing purposes by capturing the origin of the workflow trigger.
	TraceOrigin string
}

// TypedRecord differs from Record in that it contains a Typed Object and Typed Status
type TypedRecord[Type any, Status StatusType] struct {
	Record
	Status Status
	Object *Type
}
