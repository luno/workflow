package wredis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestParseStreamID(t *testing.T) {
	testCases := []struct {
		name        string
		streamID    string
		expectedID  int64
		expectError bool
		errorMsg    string
	}{
		{
			name:       "valid stream ID",
			streamID:   "1640995200000-0",
			expectedID: 1640995200000000000,
		},
		{
			name:       "valid stream ID with sequence",
			streamID:   "1640995200000-5",
			expectedID: 1640995200000000005,
		},
		{
			name:       "same timestamp different sequence",
			streamID:   "1640995200000-123",
			expectedID: 1640995200000000123,
		},
		{
			name:       "maximum valid sequence",
			streamID:   "1640995200000-999999",
			expectedID: 1640995200000999999,
		},
		{
			name:        "empty stream ID",
			streamID:    "",
			expectError: true,
			errorMsg:    "empty stream ID",
		},
		{
			name:        "invalid format - no dash",
			streamID:    "1640995200000",
			expectError: true,
			errorMsg:    "invalid stream ID format",
		},
		{
			name:        "invalid format - multiple dashes",
			streamID:    "1640995200000-0-1",
			expectError: true,
			errorMsg:    "invalid stream ID format",
		},
		{
			name:        "empty timestamp",
			streamID:    "-0",
			expectError: true,
			errorMsg:    "empty timestamp",
		},
		{
			name:        "empty sequence",
			streamID:    "1640995200000-",
			expectError: true,
			errorMsg:    "empty sequence",
		},
		{
			name:        "non-numeric timestamp",
			streamID:    "abc-0",
			expectError: true,
			errorMsg:    "invalid timestamp",
		},
		{
			name:        "non-numeric sequence",
			streamID:    "1640995200000-abc",
			expectError: true,
			errorMsg:    "invalid sequence",
		},
		{
			name:        "double dash creates invalid format",
			streamID:    "1640995200000--1",
			expectError: true,
			errorMsg:    "invalid stream ID format",
		},
		{
			name:        "sequence too large",
			streamID:    "1640995200000-1000000",
			expectError: true,
			errorMsg:    "sequence too large",
		},
		{
			name:        "timestamp too large for overflow protection",
			streamID:    "9223372036855000-0",
			expectError: true,
			errorMsg:    "timestamp too large",
		},
		{
			name:       "edge case - minimum timestamp",
			streamID:   "0-0",
			expectedID: 0,
		},
		{
			name:       "edge case - realistic timestamp with sequence",
			streamID:   "1735834800000-42", // Example: 2025-01-02 15:00:00 UTC
			expectedID: 1735834800000000042,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := parseStreamID(tc.streamID)

			if tc.expectError {
				require.Error(t, err, "Expected error for stream ID: %s", tc.streamID)
				require.Contains(t, err.Error(), tc.errorMsg,
					"Error message should contain '%s' for stream ID: %s", tc.errorMsg, tc.streamID)
			} else {
				require.NoError(t, err, "Unexpected error for stream ID: %s", tc.streamID)
				require.Equal(t, tc.expectedID, id,
					"Event ID should match expected value for stream ID: %s", tc.streamID)
			}
		})
	}
}

func TestParseStreamIDCollisionPrevention(t *testing.T) {
	// Test that different stream IDs produce different event IDs
	streamIDs := []string{
		"1640995200000-0",
		"1640995200000-1",
		"1640995200000-999",
		"1640995200001-0", // Different timestamp
	}

	seenIDs := make(map[int64]string)

	for _, streamID := range streamIDs {
		id, err := parseStreamID(streamID)
		require.NoError(t, err, "Failed to parse stream ID: %s", streamID)

		if existingStreamID, exists := seenIDs[id]; exists {
			t.Errorf("ID collision detected: stream IDs '%s' and '%s' both produce event ID %d",
				streamID, existingStreamID, id)
		}
		seenIDs[id] = streamID
	}

	require.Len(t, seenIDs, len(streamIDs), "All stream IDs should produce unique event IDs")
}

func TestParseStreamIDEdgeCases(t *testing.T) {
	// Test specific edge cases that could happen in practice
	t.Run("Test sequence bounds validation", func(t *testing.T) {
		// Test normal positive sequence handling at the boundaries:
		// - sequence 0 (minimum valid sequence)
		// - sequence 999999 (maximum allowed sequence)
		// This ensures our parsing correctly handles the full valid range

		// Test minimum valid sequence (0)
		id, err := parseStreamID("1640995200000-0")
		require.NoError(t, err)
		require.Equal(t, int64(1640995200000000000), id)

		// Test maximum allowed sequence (999999)
		id, err = parseStreamID("1640995200000-999999")
		require.NoError(t, err)
		require.Equal(t, int64(1640995200000999999), id)
	})

	t.Run("Test overflow protection", func(t *testing.T) {
		// Test that our overflow protection works
		_, err := parseStreamID("9223372036855000-0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "timestamp too large")

		// Test sequence too large
		_, err = parseStreamID("1640995200000-1000000")
		require.Error(t, err)
		require.Contains(t, err.Error(), "sequence too large")
	})
}

func TestReceiverErrorHandling(t *testing.T) {
	// This test verifies that malformed messages are handled properly
	// and not silently acknowledged
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})
	t.Cleanup(func() { _ = client.Close() })

	streamer := NewStreamer(client)
	topic := "test-malformed"

	// Create sender and receiver
	sender, err := streamer.NewSender(ctx, topic)
	require.NoError(t, err)
	defer sender.Close()

	receiver, err := streamer.NewReceiver(ctx, topic, "test-receiver")
	require.NoError(t, err)
	defer receiver.Close()

	// Manually inject malformed data directly to Redis stream
	streamKey := streamKeyPrefix + topic
	_, err = client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event": "invalid-json-data{",
		},
	}).Result()
	require.NoError(t, err)

	// Try to receive - should get an error, not silent acknowledgment
	// Use a timeout context to prevent hanging
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, _, err = receiver.Recv(recvCtx)
	require.Error(t, err, "Should return error for malformed message")
	require.Contains(t, err.Error(), "failed to parse", "Error should mention parse failure")

	// Verify the malformed message is still in the stream (not acknowledged)
	// by checking pending messages for the consumer group
	consumerGroup := consumerGroupPrefix + "test-receiver"
	pending, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pending, 1, "Malformed message should still be pending (not acknowledged)")
}

func TestPollingFrequencyImplementation(t *testing.T) {
	// Test that polling frequency is correctly used for Redis blocking
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})
	t.Cleanup(func() { _ = client.Close() })

	streamer := NewStreamer(client)
	topic := "test-polling"

	// Test with custom polling frequency
	customPollFreq := 500 * time.Millisecond
	receiver, err := streamer.NewReceiver(ctx, topic, "test-receiver",
		workflow.WithReceiverPollFrequency(customPollFreq))
	require.NoError(t, err)
	defer receiver.Close()

	// Verify the receiver uses the correct block duration
	recv := receiver.(*Receiver)
	blockDuration := recv.getBlockDuration()
	require.Equal(t, customPollFreq, blockDuration,
		"Block duration should match custom polling frequency")

	// Test default polling frequency (when not specified)
	defaultReceiver, err := streamer.NewReceiver(ctx, topic, "default-receiver")
	require.NoError(t, err)
	defer defaultReceiver.Close()

	defaultRecv := defaultReceiver.(*Receiver)
	defaultBlockDuration := defaultRecv.getBlockDuration()
	require.Equal(t, 250*time.Millisecond, defaultBlockDuration,
		"Default block duration should be 250 milliseconds")
}

func TestAckWithCancelledContext(t *testing.T) {
	// Test that ack works even after Recv context is cancelled
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})
	t.Cleanup(func() { _ = client.Close() })

	streamer := NewStreamer(client)
	topic := "test-ack-context"

	// Create sender and send a message
	sender, err := streamer.NewSender(ctx, topic)
	require.NoError(t, err)
	defer sender.Close()

	err = sender.Send(ctx, "test-message", 1, map[workflow.Header]string{
		workflow.HeaderTopic: topic,
	})
	require.NoError(t, err)

	// Create receiver with a context that we'll cancel
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	receiver, err := streamer.NewReceiver(recvCtx, topic, "test-receiver")
	require.NoError(t, err)
	defer receiver.Close()

	// Receive the message
	event, ack, err := receiver.Recv(recvCtx)
	require.NoError(t, err)
	require.NotNil(t, event)
	require.NotNil(t, ack)
	require.Equal(t, "test-message", event.ForeignID)

	// Cancel the Recv context to simulate timeout/cancellation
	cancel()

	// Wait a bit to ensure context is cancelled
	time.Sleep(10 * time.Millisecond)

	// Ack should still work because it uses a fresh background context
	err = ack()
	require.NoError(t, err, "Ack should succeed even after Recv context is cancelled")

	// Verify the message was actually acknowledged by checking pending count
	streamKey := streamKeyPrefix + topic
	consumerGroup := consumerGroupPrefix + "test-receiver"
	pending, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	require.Len(t, pending, 0, "Message should be acknowledged and not pending")
}

func TestConcurrentStreamFromLatestReceivers(t *testing.T) {
	// Test that multiple receivers with StreamFromLatest don't fail with BUSYGROUP
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})
	t.Cleanup(func() { _ = client.Close() })

	streamer := NewStreamer(client)
	topic := "test-concurrent-latest"

	// Create multiple receivers concurrently with StreamFromLatest
	const numReceivers = 5
	receivers := make([]workflow.EventReceiver, numReceivers)

	// Use a sync.WaitGroup to start all receivers at roughly the same time
	var startWg sync.WaitGroup
	errorsChan := make(chan error, numReceivers)

	startWg.Add(numReceivers)

	for i := 0; i < numReceivers; i++ {
		go func(i int) {
			defer startWg.Done()
			receiverName := fmt.Sprintf("concurrent-receiver-%d", i)
			receiver, err := streamer.NewReceiver(ctx, topic, receiverName, workflow.StreamFromLatest())
			if err != nil {
				errorsChan <- fmt.Errorf("receiver %d failed: %w", i, err)
				return
			}
			receivers[i] = receiver
		}(i)
	}

	// Wait for all receivers to be created
	startWg.Wait()
	close(errorsChan)

	// Check for any errors
	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	require.Empty(t, errors, "No receivers should fail with BUSYGROUP error: %v", errors)

	// Clean up receivers
	for i, receiver := range receivers {
		if receiver != nil {
			err := receiver.Close()
			require.NoError(t, err, "Failed to close receiver %d", i)
		}
	}
}
