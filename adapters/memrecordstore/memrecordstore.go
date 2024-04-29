package memrecordstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/luno/jettison/errors"
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
		keyIndex:         make(map[string]*workflow.WireRecord),
		store:            make(map[int64]*workflow.WireRecord),
		snapshots:        make(map[string][]*workflow.WireRecord),
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

	keyIndex map[string]*workflow.WireRecord
	store    map[int64]*workflow.WireRecord

	outbox            []workflow.OutboxEvent
	outboxIDIncrement int64

	snapshots        map[string][]*workflow.WireRecord
	snapshotsOffsets map[string]int
}

func (s *Store) Lookup(ctx context.Context, id int64) (*workflow.WireRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.store[id]
	if !ok {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	}

	// Return a new pointer so modifications don't affect the store.
	return &workflow.WireRecord{
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

func (s *Store) Store(ctx context.Context, record *workflow.WireRecord, maker workflow.OutboxEventDataMaker) error {
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

	snapshotKey := fmt.Sprintf("%v-%v-%v", record.WorkflowName, record.ForeignID, record.RunID)
	s.snapshots[snapshotKey] = append(s.snapshots[snapshotKey], record)

	return nil
}

func (s *Store) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.WireRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uk := uniqueKey(workflowName, foreignID)
	record, ok := s.keyIndex[uk]
	if !ok {
		return nil, errors.Wrap(workflow.ErrRecordNotFound, "")
	}

	// Return a new pointer so modifications don't affect the store.
	return &workflow.WireRecord{
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

func (s *Store) List(ctx context.Context, workflowName string, offsetID int64, limit int, order workflow.OrderType) ([]workflow.WireRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		entries []workflow.WireRecord
		length  = int64(len(s.store))
		start   = offsetID
		end     = start + int64(limit)
	)

	for i := start + 1; i <= end; i++ {
		if i > length {
			break
		}

		if len(entries)+1 > limit {
			break
		}

		entry, ok := s.store[i]
		if !ok {
			continue
		}

		entries = append(entries, *entry)
	}

	if order == workflow.OrderTypeDescending {
		var descEntries []workflow.WireRecord
		for i := len(entries) - 1; i >= 0; i-- {
			descEntries = append(descEntries, entries[i])
		}

		return descEntries, nil
	}

	return entries, nil
}

func (s *Store) Snapshots(workflowName, foreignID, runID string) []*workflow.WireRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%v-%v-%v", workflowName, foreignID, runID)
	return s.snapshots[key]
}

func (s *Store) SetSnapshotOffset(workflowName, foreignID, runID string, offset int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%v-%v-%v", workflowName, foreignID, runID)
	s.snapshotsOffsets[key] = offset
}

func (s *Store) SnapshotOffset(workflowName, foreignID, runID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%v-%v-%v", workflowName, foreignID, runID)
	return s.snapshotsOffsets[key]
}

func uniqueKey(s1, s2 string) string {
	return fmt.Sprintf("%v-%v", s1, s2)
}
