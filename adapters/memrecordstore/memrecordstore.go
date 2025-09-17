package memrecordstore

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
)

const defaultListLimit = 25

// New constructs and returns an in-memory Store configured by the provided options.
// 
// The returned Store holds workflows, snapshots and an optional outbox in memory.
// Callers may pass Option functions to override defaults (the default clock is the real clock).
//
// If WithOutbox was used to enable outbox processing, New will panic if the required
// outbox dependencies (context, event streamer or logger) are nil. When enabled it
// starts a background goroutine that continuously purges and publishes outbox events;
// that goroutine honours context cancellation and logs unexpected termination.
//
// The function returns a pointer to the initialised Store.
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
		store:            make(map[string]*workflow.Record),
		snapshots:        make(map[string][]*workflow.Record),
		snapshotsOffsets: make(map[string]int),
		clock:            opt.clock,
	}

	if opt.writeToOutbox {
		if opt.ctx == nil || opt.eventStreamer == nil || opt.logger == nil {
			panic("outbox requirements not fully satisfied (ctx, event streamer, or logger is nil)")
		}

		go func() {
			err := PurgeOutboxForever(
				opt.ctx,
				s.listOutbox,
				s.DeleteOutboxEvent,
				opt.eventStreamer,
				opt.logger,
				10*time.Millisecond,
				1000,
			)
			if errors.Is(err, context.Canceled) {
				opt.logger.Debug(opt.ctx, "outbox processor stopped", map[string]string{})
			} else if err != nil {
				err = errors.Join(
					errors.New("outbox processor stopped with unexpectedly"),
					err,
				)
				opt.logger.Error(opt.ctx, err)
			}
		}()
	}

	return s
}

type options struct {
	clock clock.Clock

	writeToOutbox bool
	ctx           context.Context
	eventStreamer workflow.EventStreamer
	logger        workflow.Logger
}

type Option func(o *options)

// WithOutbox returns an Option that enables background outbox processing for the store
// using the provided context, EventStreamer and Logger. When this option is applied the
// store will record outbox events and New will start a background goroutine to purge
// and process them. New will panic if any of ctx, es or logger are nil.
func WithOutbox(ctx context.Context, es workflow.EventStreamer, logger workflow.Logger) Option {
	return func(o *options) {
		o.writeToOutbox = true
		o.ctx = ctx
		o.eventStreamer = es
		o.logger = logger
	}
}

// WithClock returns an Option that sets the clock implementation used by the store.
// Use this to override the default real-time clock (for example in tests or when
// injecting a custom time source).
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
	store    map[string]*workflow.Record
	order    []string

	outbox            []workflow.OutboxEvent
	outboxIDIncrement int64

	snapshots        map[string][]*workflow.Record
	snapshotsOffsets map[string]int
}

func (s *Store) Lookup(ctx context.Context, id string) (*workflow.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.store[id]
	if !ok {
		return nil, workflow.ErrRecordNotFound
	}

	// Return a new pointer so modifications don't affect the store.
	return &workflow.Record{
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     record.RunState,
		Status:       record.Status,
		Object:       record.Object,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Meta:         record.Meta,
	}, nil
}

func (s *Store) Store(ctx context.Context, record *workflow.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add record to store
	uk := uniqueKey(record.WorkflowName, record.ForeignID)
	s.keyIndex[uk] = record

	eventData, err := workflow.MakeOutboxEventData(*record)
	if err != nil {
		return err
	}

	_, previouslyExisted := s.store[record.RunID]
	if !previouslyExisted {
		s.order = append(s.order, record.RunID)
	}
	s.store[record.RunID] = record
	s.outbox = append(s.outbox, workflow.OutboxEvent{
		ID:           eventData.ID,
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
		WorkflowName: record.WorkflowName,
		ForeignID:    record.ForeignID,
		RunID:        record.RunID,
		RunState:     record.RunState,
		Status:       record.Status,
		Object:       record.Object,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		Meta:         record.Meta,
	}, nil
}

func (s *Store) ListOutboxEvents(
	ctx context.Context,
	workflowName string,
	limit int64,
) ([]workflow.OutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []workflow.OutboxEvent
	for _, outboxEvent := range s.outbox {
		if outboxEvent.WorkflowName != workflowName {
			continue
		}

		filtered = append(filtered, outboxEvent)
		if len(filtered) >= int(limit) {
			break
		}
	}

	return filtered, nil
}

func (s *Store) listOutbox(
	limit int64,
) ([]workflow.OutboxEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []workflow.OutboxEvent
	for _, outboxEvent := range s.outbox {

		filtered = append(filtered, outboxEvent)
		if len(filtered) >= int(limit) {
			break
		}
	}

	return filtered, nil
}

func (s *Store) DeleteOutboxEvent(ctx context.Context, id string) error {
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

func (s *Store) List(
	ctx context.Context,
	workflowName string,
	offset int64,
	limit int,
	order workflow.OrderType,
	filters ...workflow.RecordFilter,
) ([]workflow.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit == 0 {
		limit = defaultListLimit
	}

	filter := workflow.MakeFilter(filters...)
	filteredStore := make(map[int64]*workflow.Record)
	increment := int64(1)
	for _, runID := range s.order {
		record, ok := s.store[runID]
		if !ok {
			continue
		}

		if workflowName != "" && workflowName != record.WorkflowName {
			continue
		}

		if filter.ByForeignID().Enabled && !filter.ByForeignID().Matches(record.ForeignID) {
			continue
		}

		status := strconv.FormatInt(int64(record.Status), 10)
		if filter.ByStatus().Enabled && !filter.ByStatus().Matches(status) {
			continue
		}

		runState := strconv.FormatInt(int64(record.RunState), 10)
		if filter.ByRunState().Enabled && !filter.ByRunState().Matches(runState) {
			continue
		}

		if filter.ByCreatedAtAfter().Enabled && !filter.ByCreatedAtAfter().Matches(record.CreatedAt) {
			continue
		}
		if filter.ByCreatedAtBefore().Enabled && !filter.ByCreatedAtBefore().Matches(record.CreatedAt) {
			continue
		}

		filteredStore[increment] = record
		increment++
	}

	var (
		entries []workflow.Record
		length  = int64(len(filteredStore))
		start   = offset + 1
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
