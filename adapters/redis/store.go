package redis

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/luno/workflow"
)

const (
	defaultListLimit = 25
	// Key prefixes
	recordKeyPrefix = "workflow:record:"
	indexKeyPrefix  = "workflow:index:"
	outboxKeyPrefix = "workflow:outbox:"
)

type Store struct {
	client redis.UniversalClient
}

func New(client redis.UniversalClient) *Store {
	return &Store{
		client: client,
	}
}

var _ workflow.RecordStore = (*Store)(nil)

// Lua scripts for atomic operations
var (
	storeScript = redis.NewScript(`
		local record_key = KEYS[1]
		local index_key = KEYS[2]
		local list_key = KEYS[3]
		local global_list_key = KEYS[4]
		local outbox_key = KEYS[5]

		local record_data = ARGV[1]
		local run_id = ARGV[2]
		local score = ARGV[3]
		local outbox_data = ARGV[4]

		-- Store record
		redis.call('SET', record_key, record_data)

		-- Update index
		redis.call('SET', index_key, run_id)

		-- Add to sorted sets for listing
		redis.call('ZADD', list_key, score, run_id)
		redis.call('ZADD', global_list_key, score, run_id)

		-- Add to outbox
		redis.call('LPUSH', outbox_key, outbox_data)

		return 'OK'
	`)

	deleteOutboxScript = redis.NewScript(`
		local outbox_key = KEYS[1]
		local outbox_id = ARGV[1]

		-- Get all outbox events
		local events = redis.call('LRANGE', outbox_key, 0, -1)

		-- Find and remove the event with matching ID
		for i, event_data in ipairs(events) do
			local event = cjson.decode(event_data)
			if event.ID == outbox_id then
				-- Remove this element
				redis.call('LREM', outbox_key, 1, event_data)
				return 1
			end
		end

		return 0
	`)
)

// Store implements the RecordStore interface with outbox pattern
func (s *Store) Store(ctx context.Context, record *workflow.Record) error {
	// Generate outbox event data
	eventData, err := workflow.MakeOutboxEventData(*record)
	if err != nil {
		return err
	}

	outboxEvent := &workflow.OutboxEvent{
		ID:           eventData.ID,
		WorkflowName: eventData.WorkflowName,
		Data:         eventData.Data,
		CreatedAt:    time.Now(),
	}

	// Serialize just the record for storage
	recordData, err := json.Marshal(record)
	if err != nil {
		return err
	}

	// Serialize outbox event for outbox storage
	outboxData, err := json.Marshal(outboxEvent)
	if err != nil {
		return err
	}

	// Prepare keys and arguments for Lua script
	recordKey := recordKeyPrefix + record.RunID
	indexKey := indexKeyPrefix + record.WorkflowName + ":" + record.ForeignID
	listKey := "workflow:list:" + record.WorkflowName
	globalListKey := "workflow:list:all"
	outboxKey := outboxKeyPrefix + record.WorkflowName

	score := strconv.FormatFloat(float64(record.CreatedAt.Unix()), 'f', -1, 64)

	// Execute Lua script atomically
	return storeScript.Run(ctx, s.client,
		[]string{recordKey, indexKey, listKey, globalListKey, outboxKey},
		string(recordData), record.RunID, score, string(outboxData)).Err()
}

// Lookup implements the RecordStore interface
func (s *Store) Lookup(ctx context.Context, runID string) (*workflow.Record, error) {
	recordKey := recordKeyPrefix + runID
	data, err := s.client.Get(ctx, recordKey).Result()
	if err == redis.Nil {
		return nil, workflow.ErrRecordNotFound
	} else if err != nil {
		return nil, err
	}

	var record workflow.Record
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// Latest implements the RecordStore interface
func (s *Store) Latest(ctx context.Context, workflowName, foreignID string) (*workflow.Record, error) {
	indexKey := indexKeyPrefix + workflowName + ":" + foreignID
	runID, err := s.client.Get(ctx, indexKey).Result()
	if err == redis.Nil {
		return nil, workflow.ErrRecordNotFound
	} else if err != nil {
		return nil, err
	}

	return s.Lookup(ctx, runID)
}

// List implements the RecordStore interface
func (s *Store) List(ctx context.Context, workflowName string, offset int64, limit int, order workflow.OrderType, filters ...workflow.RecordFilter) ([]workflow.Record, error) {
	if limit == 0 {
		limit = defaultListLimit
	}

	var listKey string
	if workflowName == "" {
		listKey = "workflow:list:all"
	} else {
		listKey = "workflow:list:" + workflowName
	}

	filter := workflow.MakeFilter(filters...)

	// When filters are applied, we need to fetch more records than requested
	// to account for filtering, then apply offset and limit after filtering
	fetchLimit := int64(limit)
	fetchOffset := offset

	// If we have filters, we might need to fetch more records
	if filter.ByForeignID().Enabled || filter.ByStatus().Enabled || filter.ByRunState().Enabled ||
	   filter.ByCreatedAtAfter().Enabled || filter.ByCreatedAtBefore().Enabled {
		// Increase fetch limit to account for filtering
		// This is a heuristic - in production, you'd want more sophisticated logic
		fetchLimit = int64(limit * 3) // Fetch 3x more records
		fetchOffset = 0 // Start from beginning when filtering
	}

	// Determine Redis command based on order
	var runIDs []string
	var err error

	if order == workflow.OrderTypeDescending {
		// Get in reverse chronological order (newest first)
		runIDs, err = s.client.ZRevRange(ctx, listKey, fetchOffset, fetchOffset+fetchLimit-1).Result()
	} else {
		// Get in chronological order (oldest first)
		runIDs, err = s.client.ZRange(ctx, listKey, fetchOffset, fetchOffset+fetchLimit-1).Result()
	}

	if err != nil {
		return nil, err
	}

	var allRecords []workflow.Record

	for _, runID := range runIDs {
		record, err := s.Lookup(ctx, runID)
		if err != nil {
			continue // Skip missing records
		}

		// Apply filters
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

		allRecords = append(allRecords, *record)
	}

	// Apply offset and limit after filtering when filters are present
	if filter.ByForeignID().Enabled || filter.ByStatus().Enabled || filter.ByRunState().Enabled ||
	   filter.ByCreatedAtAfter().Enabled || filter.ByCreatedAtBefore().Enabled {

		start := int(offset)
		if start > len(allRecords) {
			return []workflow.Record{}, nil
		}

		end := start + limit
		if end > len(allRecords) {
			end = len(allRecords)
		}

		return allRecords[start:end], nil
	}

	return allRecords, nil
}

// ListOutboxEvents implements the RecordStore interface
func (s *Store) ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]workflow.OutboxEvent, error) {
	outboxKey := outboxKeyPrefix + workflowName

	// Get all outbox events from the list (limited by count)
	eventStrings, err := s.client.LRange(ctx, outboxKey, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	var events []workflow.OutboxEvent
	for _, eventStr := range eventStrings {
		var event workflow.OutboxEvent
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			continue // Skip malformed events
		}
		events = append(events, event)
	}

	return events, nil
}

// DeleteOutboxEvent implements the RecordStore interface
func (s *Store) DeleteOutboxEvent(ctx context.Context, id string) error {
	// We need to check all workflow outbox lists to find the event with this ID
	// Since we don't know which workflow this event belongs to, we'll need to check all of them
	// For production use, you might want to maintain a reverse index or store workflow name with the event ID

	// Get all outbox keys (this is not ideal for production - would be better to maintain an index)
	pattern := outboxKeyPrefix + "*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	// Try to delete from each outbox list using the Lua script
	for _, outboxKey := range keys {
		result, err := deleteOutboxScript.Run(ctx, s.client, []string{outboxKey}, id).Int()
		if err != nil {
			continue // Try next outbox
		}
		if result == 1 {
			// Successfully found and deleted the event
			return nil
		}
	}

	// Event not found in any outbox - this is not necessarily an error
	return nil
}