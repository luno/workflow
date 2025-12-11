package workflow

import (
	"context"
	"sync"
)

// Run is a representation of a workflow run. It incorporates all the fields from the Record as well as
// having defined types for the Status and Object fields along with access to the RunStateController which
// controls the state of the run aka "RunState".
type Run[Type any, Status StatusType] struct {
	TypedRecord[Type, Status]

	// stopper provides controls over the run state of the record. Run is not serializable and is not
	// intended to be and thus Record exists as a serializable representation of a record.
	controller RunStateController
}

// Pause is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Pause(ctx)" to pause a record from inside a workflow which results in the record
// being temporarily left alone and will not be processed until it is resumed.
func (r *Run[Type, Status]) Pause(ctx context.Context, reason string) (Status, error) {
	err := r.controller.Pause(ctx, reason)
	if err != nil {
		return 0, err
	}

	return Status(skipTypeRunStateUpdate), nil
}

// Skip is a util function to skip the update and move on to the next event (consumer) or execution (callback)
func (r *Run[Type, Status]) Skip() (Status, error) {
	return Status(skipTypeDefault), nil
}

// Cancel is intended to be used inside a workflow process where (Status, error) are the return signature. This allows
// the user to simply type "return r.Cancel(ctx)" to cancel a record from inside a workflow which results in the record
// being permanently left alone and will not be processed.
func (r *Run[Type, Status]) Cancel(ctx context.Context, reason string) (Status, error) {
	err := r.controller.Cancel(ctx, reason)
	if err != nil {
		return 0, err
	}

	return Status(skipTypeRunStateUpdate), nil
}

func (r *Run[Type, Status]) SaveAndRepeat() (Status, error) {
	return Status(skipTypeSaveAndRepeat), nil
}

type (
	runCollector[Type any, Status StatusType] func() *Run[Type, Status]
	runReleaser[Type any, Status StatusType]  func(*Run[Type, Status])
)

// newRunPool creates a new sync.Pool for Run objects with 10 pre-allocated instances
func newRunPool[Type any, Status StatusType]() *sync.Pool {
	pool := sync.Pool{
		New: func() interface{} {
			return &Run[Type, Status]{}
		},
	}

	// Pre-allocate 10 Run objects in the pool for better performance
	for i := 0; i < 10; i++ {
		pool.Put(&Run[Type, Status]{})
	}

	return &pool
}

func buildRun[Type any, Status StatusType](collector runCollector[Type, Status], store storeFunc, wr *Record) (*Run[Type, Status], error) {
	var t Type
	err := Unmarshal(wr.Object, &t)
	if err != nil {
		return nil, err
	}

	// The first time the record is consumed, it needs to be marked as RunStateRunning to represent that the record
	// has begun being processed. Even if the consumer errors then this should update should remain in place and
	// not be executed on the subsequent retries.
	if wr.RunState == RunStateInitiated {
		wr.RunState = RunStateRunning
	}

	// Get Run from pool and initialize
	run := collector()

	// Reset/initialize the run object
	controller := NewRunStateController(store, wr)
	run.TypedRecord = TypedRecord[Type, Status]{
		Record: *wr,
		Status: Status(wr.Status),
		Object: &t,
	}
	run.controller = controller

	return run, nil
}

// newRunObj returns a function that gets a Run from the pool
func (w *Workflow[Type, Status]) newRunObj() runCollector[Type, Status] {
	return func() *Run[Type, Status] {
		return w.runPool.Get().(*Run[Type, Status])
	}
}

// releaseRun returns a Run object back to the workflow's pool for reuse
func (w *Workflow[Type, Status]) releaseRun(run *Run[Type, Status]) {
	// Clear references to prevent memory leaks
	run.Object = nil
	run.controller = nil
	// Note: We don't clear the Record as it's a value type

	w.runPool.Put(run)
}
