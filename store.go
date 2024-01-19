package workflow

import (
	"context"
	"time"
)

// RecordStore implementations should all be tested with adaptertest.TestRecordStore
type RecordStore interface {
	// Store should create or update a record depending on whether the underlying store is mutable or append only. Store
	// should implement transactions if it is supported especially if the Store is append-only as a new ID for the
	// record will need to be passed to the event emitter.
	Store(ctx context.Context, record *WireRecord, eventEmitter EventEmitter) error
	Lookup(ctx context.Context, id int64) (*WireRecord, error)
	Latest(ctx context.Context, workflowName, foreignID string) (*WireRecord, error)
}

type TestingRecordStore interface {
	RecordStore

	Snapshots(workflowName, foreignID, runID string) []*WireRecord
	SetSnapshotOffset(workflowName, foreignID, runID string, offset int)
	SnapshotOffset(workflowName, foreignID, runID string) int
}

// EventEmitter is a function that gets called before committing the change to the store. The store needs to support
// transactions if it is implemented as an append only datastore to allow rolling back if the event fails to emit.
type EventEmitter func(id int64) error

// TimeoutStore implementations should all be tested with adaptertest.TestTimeoutStore
type TimeoutStore interface {
	Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error
	Complete(ctx context.Context, id int64) error
	Cancel(ctx context.Context, id int64) error
	List(ctx context.Context, workflowName string) ([]Timeout, error)
	ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]Timeout, error)
}
