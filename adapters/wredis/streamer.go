package wredis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/luno/workflow"
	"github.com/redis/go-redis/v9"
)

const (
	streamKeyPrefix = "workflow:stream:"
	cursorKeyPrefix = "workflow:cursor:"

	consumerGroupPrefix = "workflow-"
)

type Streamer struct {
	client redis.UniversalClient
	mu     sync.RWMutex
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
			if err != nil {
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
		// Try to read pending messages first
		pendingMsgs, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: r.name,
			Streams:  []string{streamKey, ">"},
			Count:    1,
			Block:    100 * time.Millisecond,
		}).Result()

		if err != nil && err != redis.Nil {
			return nil, nil, err
		}

		if len(pendingMsgs) > 0 && len(pendingMsgs[0].Messages) > 0 {
			msg := pendingMsgs[0].Messages[0]
			event, err := r.parseEvent(msg)
			if err != nil {
				// Acknowledge bad message and continue
				r.client.XAck(ctx, streamKey, consumerGroup, msg.ID)
				continue
			}

			ack := func() error {
				return r.client.XAck(ctx, streamKey, consumerGroup, msg.ID).Err()
			}

			return event, ack, nil
		}

		// If no messages and using polling frequency, wait
		if r.options.PollFrequency > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(r.options.PollFrequency):
				continue
			}
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


func (r *Receiver) Close() error {
	return nil
}

// parseStreamID converts Redis Stream ID (timestamp-sequence) to int64
func parseStreamID(streamID string) (int64, error) {
	// Redis Stream ID format: "timestamp-sequence"
	// We'll use just the timestamp part as the event ID
	if streamID == "" {
		return 0, fmt.Errorf("empty stream ID")
	}

	// Find the dash separator
	dashIndex := -1
	for i, char := range streamID {
		if char == '-' {
			dashIndex = i
			break
		}
	}

	if dashIndex == -1 {
		return 0, fmt.Errorf("invalid stream ID format: %s", streamID)
	}

	timestamp := streamID[:dashIndex]
	return strconv.ParseInt(timestamp, 10, 64)
}
