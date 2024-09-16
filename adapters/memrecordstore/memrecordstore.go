package memrecordstore

import (
	"context"
	"strconv"
	"sync"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
)

func New(opts ...Option) *Store {
	// Set option defaults
	opt := options{
		clock: clock.RealClock{},
	}

	// Set option overrides
	for _, o := range opts {
		o(&opt)
	}

	s := &Store{
		keyIndex:         make(map[string]*workflow.Record),
		store:            make(map[int64]*workflow.Record),
		snapshots:        make(map[string][]*workflow.Record),
		snapshotsOffsets: make(map[string]int),
		clock:            opt.clock,
	}

	return s
}

type options struct {
	clock clock.Clock
}

type Option func(o *options)

func WithClock(clock clock.Clock) Option {
	return func(o *options) {
		o.clock = clock
	}
}

var _ workflow.RecordStore = (*Store)(nil)

type Store struct {
	mu          sync.Mutex
	idIncrement int64

	clock clock.Clock

	keyIndex map[string]*workflow.Record
	store    map[int64]*workflow.Record

	outbox            []workflow.OutboxEvent
	outboxIDIncrement int64

	snapshots        map[string][]*workflow.Record
	snapshotsOffsets map[string]int
}

func (s *Store) Lookup(ctx context.Context, id int64) (*workflow.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.store[id]
	if !ok {
		return nil, workflow.ErrRecordNotFound
	}

	// Return a new pointer so modifications don't affect the store.
	return &workflow.Record{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     record.RunState,
		Status:       record.Status,
		Object:       record.Object,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}, nil
}

func (s *Store) Store(ctx context.Context, record *workflow.Record, maker workflow.OutboxEventDataMaker) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == 0 {
		s.idIncrement++
		record.ID = s.idIncrement
	}

	// Add record to store
	uk := uniqueKey(record.WorkflowName, record.ForeignID)
	s.keyIndex[uk] = record
	s.store[record.ID] = record

	// Add event to outbox
	eventData, err := maker(record.ID)
	if err != nil {
		return err
	}

	s.outboxIDIncrement++
	s.outbox = append(s.outbox, workflow.OutboxEvent{
		ID:           s.outboxIDIncrement,
		WorkflowName: eventData.WorkflowName,
		Data:         eventData.Data,
		CreatedAt:    s.clock.Now(),
	})

	skey := snapShotKey(record.WorkflowName, record.ForeignID, record.RunID)
	s.snapshots[skey] = append(s.snapshots[skey], record)

	return nil
}

func (s *Store) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uk := uniqueKey(workflowName, foreignID)
	record, ok := s.keyIndex[uk]
	if !ok {
		return nil, workflow.ErrRecordNotFound
	}

	// Return a new pointer so modifications don't affect the store.
	return &workflow.Record{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     record.RunState,
		Status:       record.Status,
		Object:       record.Object,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}, nil
}

func (s *Store) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]workflow.OutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []workflow.OutboxEvent
	for _, outboxEvent := range s.outbox {
		if outboxEvent.WorkflowName != workflowName {
			continue
		}

		filtered = append(filtered, outboxEvent)
	}

	return filtered, nil
}

func (s *Store) DeleteOutboxEvent(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []workflow.OutboxEvent
	for _, outboxEvent := range s.outbox {
		if outboxEvent.ID == id {
			continue
		}

		filtered = append(filtered, outboxEvent)
	}

	s.outbox = filtered
	return nil
}

func (s *Store) List(ctx context.Context, workflowName string, offsetID int64, limit int, order workflow.OrderType, filters ...workflow.RecordFilter) ([]workflow.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := workflow.MakeFilter(filters...)
	filteredStore := make(map[int64]*workflow.Record)
	increment := int64(1)
	if len(filters) > 0 {
		for _, record := range s.store {
			if filter.ByForeignID().Enabled && filter.ByForeignID().Value != record.ForeignID {
				continue
			}

			status := strconv.FormatInt(int64(record.Status), 10)
			if filter.ByStatus().Enabled && filter.ByStatus().Value != status {
				continue
			}

			runState := strconv.FormatInt(int64(record.RunState), 10)
			if filter.ByRunState().Enabled && filter.ByRunState().Value != runState {
				continue
			}

			filteredStore[increment] = record
			increment++
		}
	} else {
		// If no filters are specified then assign the whole store
		filteredStore = s.store
	}

	var (
		entries []workflow.Record
		length  = int64(len(filteredStore))
		start   = offsetID + 1
		end     = start + int64(limit)
	)

	for i := start; i <= end; i++ {
		if i > length {
			break
		}

		if len(entries) >= limit {
			break
		}

		entry, ok := filteredStore[i]
		if !ok {
			continue
		}

		entries = append(entries, *entry)
	}

	if order == workflow.OrderTypeDescending {
		var descEntries []workflow.Record
		for i := len(entries) - 1; i >= 0; i-- {
			descEntries = append(descEntries, entries[i])
		}

		return descEntries, nil
	}

	return entries, nil
}

func (s *Store) Snapshots(workflowName, foreignID, runID string) []*workflow.Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := snapShotKey(workflowName, foreignID, runID)
	return s.snapshots[key]
}

func (s *Store) SetSnapshotOffset(workflowName, foreignID, runID string, offset int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := snapShotKey(workflowName, foreignID, runID)
	s.snapshotsOffsets[key] = offset
}

func (s *Store) SnapshotOffset(workflowName, foreignID, runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := snapShotKey(workflowName, foreignID, runID)
	return s.snapshotsOffsets[key]
}

func snapShotKey(workflowName, foreignID, runID string) string {
	return workflowName + "-" + foreignID + "-" + runID
}

func uniqueKey(s1, s2 string) string {
	return s1 + "-" + s2
}
