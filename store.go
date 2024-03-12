package workflow

import (
	"context"
	"time"
)

// RecordStore implementations should all be tested with adaptertest.TestRecordStore. The underlying implementation of
// store must support transactions or the ability to commit the record and an outbox event in a single call as well as
// being able to obtain an ID for the record before it is created.
type RecordStore interface {
	// Store should create or update a record depending on whether the underlying store is mutable or append only. Store
	// must implement transactions and a separate outbox store to store the event that can be retrieved when calling
	// ListOutboxEvents and can be deleted when DeleteOutboxEvent is called.
	Store(ctx context.Context, record *WireRecord, maker OutboxEventDataMaker) error
	Lookup(ctx context.Context, id int64) (*WireRecord, error)
	Latest(ctx context.Context, workflowName, foreignID string) (*WireRecord, error)

	// ListOutboxEvents lists all events that are yet to be published to the event streamer. A requirement for
	// implementation of the RecordStore is to support a Transactional Outbox that has Event's written to it when
	// Store is called.
	ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]OutboxEvent, error)
	// DeleteOutboxEvent will expect an Event's ID field and will remove the event from the outbox store when the
	// event has successfully been published to the event streamer.
	DeleteOutboxEvent(ctx context.Context, id int64) error
}

type TestingRecordStore interface {
	RecordStore

	Snapshots(workflowName, foreignID, runID string) []*WireRecord
	SetSnapshotOffset(workflowName, foreignID, runID string, offset int)
	SnapshotOffset(workflowName, foreignID, runID string) int
}

// OutboxEventDataMaker is a function that constructs the expected structure of an outbox event used for creating an
// event for the event streamer. The only thing for the Store to implement is passing through the ID of the record. If
// the record is new then the ID would be obtained via the transaction (as stated in documentation for RecordStore the
// underlying store must support transactions or similar).
type OutboxEventDataMaker func(recordID int64) (OutboxEventData, error)

// TimeoutStore implementations should all be tested with adaptertest.TestTimeoutStore
type TimeoutStore interface {
	Create(ctx context.Context, workflowName, foreignID, runID string, status int, expireAt time.Time) error
	Complete(ctx context.Context, id int64) error
	Cancel(ctx context.Context, id int64) error
	List(ctx context.Context, workflowName string) ([]Timeout, error)
	ListValid(ctx context.Context, workflowName string, status int, now time.Time) ([]Timeout, error)
}
