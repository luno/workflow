package wredis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/luno/workflow"
	"github.com/redis/go-redis/v9"
)

const (
	streamKeyPrefix     = "workflow:stream:"
	consumerGroupPrefix = "workflow:consumer:"
)

type Streamer struct {
	client redis.UniversalClient
}

func NewStreamer(client redis.UniversalClient) *Streamer {
	return &Streamer{
		client: client,
	}
}

var _ workflow.EventStreamer = (*Streamer)(nil)

func (s *Streamer) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	return &Sender{
		client: s.client,
		topic:  topic,
	}, nil
}

func (s *Streamer) NewReceiver(ctx context.Context, topic string, name string, opts ...workflow.ReceiverOption) (workflow.EventReceiver, error) {
	var options workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &Receiver{
		client:  s.client,
		topic:   topic,
		name:    name,
		options: options,
	}, nil
}

type Sender struct {
	client redis.UniversalClient
	topic  string
}

var _ workflow.EventSender = (*Sender)(nil)

func (s *Sender) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	event := &workflow.Event{
		ForeignID: foreignID,
		Type:      statusType,
		Headers:   headers,
		CreatedAt: time.Now(),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	streamKey := streamKeyPrefix + s.topic

	// Use XADD to add event to Redis Stream
	_, err = s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	}).Result()

	return err
}

func (s *Sender) Close() error {
	return nil
}

type Receiver struct {
	client  redis.UniversalClient
	topic   string
	name    string
	options workflow.ReceiverOptions
}

var _ workflow.EventReceiver = (*Receiver)(nil)

func (r *Receiver) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	streamKey := streamKeyPrefix + r.topic
	consumerGroup := consumerGroupPrefix + r.name

	// Handle StreamFromLatest option by checking if consumer group exists
	if r.options.StreamFromLatest {
		// Try to get consumer group info to see if it exists
		groups, err := r.client.XInfoGroups(ctx, streamKey).Result()
		if err != nil && err.Error() != "ERR no such key" {
			return nil, nil, err
		}

		groupExists := false
		for _, group := range groups {
			if group.Name == consumerGroup {
				groupExists = true
				break
			}
		}

		if !groupExists {
			// Consumer group doesn't exist, create it starting from the latest message
			// Use "$" to start from the end of the stream
			_, err := r.client.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "$").Result()
			if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
				return nil, nil, err
			}
		}
	} else {
		// Create consumer group if it doesn't exist, starting from beginning
		_, err := r.client.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "0").Result()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return nil, nil, err
		}
	}

	for ctx.Err() == nil {
		// First, try to read pending messages that were delivered but not acknowledged
		pendingMsgs, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: r.name,
			Streams:  []string{streamKey, "0"}, // "0" reads pending messages
			Count:    1,
			Block:    0, // Don't block for pending messages
		}).Result()

		if err != nil && err != redis.Nil {
			return nil, nil, err
		}

		// Process pending message if found
		if len(pendingMsgs) > 0 && len(pendingMsgs[0].Messages) > 0 {
			msg := pendingMsgs[0].Messages[0]
			event, err := r.parseEvent(msg)
			if err != nil {
				// Return error to bubble up rather than silent ack
				return nil, nil, fmt.Errorf("failed to parse pending message %s: %w", msg.ID, err)
			}

			ack := func() error {
				// Use fresh background context with short timeout for acknowledgment
				// This allows ack to succeed even if Recv context was cancelled
				ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return r.client.XAck(ackCtx, streamKey, consumerGroup, msg.ID).Err()
			}

			return event, ack, nil
		}

		// No pending messages, try to read new messages
		// Use Redis native blocking based on polling frequency
		blockDuration := r.getBlockDuration()

		newMsgs, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: r.name,
			Streams:  []string{streamKey, ">"}, // ">" reads new messages only
			Count:    1,
			Block:    blockDuration,
		}).Result()

		if err != nil && err != redis.Nil {
			return nil, nil, err
		}

		// Process new message if found
		if len(newMsgs) > 0 && len(newMsgs[0].Messages) > 0 {
			msg := newMsgs[0].Messages[0]
			event, err := r.parseEvent(msg)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse new message %s: %w", msg.ID, err)
			}

			ack := func() error {
				// Use fresh background context with short timeout for acknowledgment
				// This allows ack to succeed even if Recv context was cancelled
				ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return r.client.XAck(ackCtx, streamKey, consumerGroup, msg.ID).Err()
			}

			return event, ack, nil
		}
	}

	return nil, nil, ctx.Err()
}

func (r *Receiver) parseEvent(msg redis.XMessage) (*workflow.Event, error) {
	eventData, ok := msg.Values["event"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid event data format")
	}

	var event workflow.Event
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		return nil, err
	}

	// Parse Redis Stream ID to set Event ID
	id, err := parseStreamID(msg.ID)
	if err != nil {
		return nil, err
	}
	event.ID = id

	return &event, nil
}

func (r *Receiver) getBlockDuration() time.Duration {
	// If PollFrequency is set, use it for blocking
	if r.options.PollFrequency > 0 {
		return r.options.PollFrequency
	}

	// Default: use a reasonable block time for real-time streaming
	return 250 * time.Millisecond
}

func (r *Receiver) Close() error {
	return nil
}

// parseStreamID converts Redis Stream ID (timestamp-sequence) to int64
func parseStreamID(streamID string) (int64, error) {
	// Redis Stream ID format: "timestamp-sequence"
	// Combine both parts to ensure uniqueness: id = timestamp*1_000_000 + sequence
	if streamID == "" {
		return 0, fmt.Errorf("empty stream ID")
	}

	// Split on the dash separator
	parts := strings.Split(streamID, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid stream ID format: %s", streamID)
	}

	timestampStr := parts[0]
	sequenceStr := parts[1]

	// Validate parts are not empty
	if timestampStr == "" {
		return 0, fmt.Errorf("empty timestamp in stream ID: %s", streamID)
	}
	if sequenceStr == "" {
		return 0, fmt.Errorf("empty sequence in stream ID: %s", streamID)
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp in stream ID %s: %w", streamID, err)
	}

	// Parse sequence
	sequence, err := strconv.ParseInt(sequenceStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid sequence in stream ID %s: %w", streamID, err)
	}

	// Check for potential overflow before combining
	// We need timestamp*1_000_000 + sequence to fit in int64
	// Max int64 is 9,223,372,036,854,775,807
	// So timestamp should not exceed 9,223,372,036,854
	const maxTimestamp = 9223372036854
	if timestamp > maxTimestamp {
		return 0, fmt.Errorf("timestamp too large in stream ID %s: would cause overflow", streamID)
	}
	if sequence >= 1000000 {
		return 0, fmt.Errorf("sequence too large in stream ID %s: must be less than 1,000,000", streamID)
	}
	if sequence < 0 {
		return 0, fmt.Errorf("sequence cannot be negative in stream ID %s", streamID)
	}

	// Combine timestamp and sequence to create unique ID
	combinedID := timestamp*1000000 + sequence
	return combinedID, nil
}
