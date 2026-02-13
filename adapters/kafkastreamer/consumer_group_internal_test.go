package kafkastreamer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/luno/workflow"
	"github.com/stretchr/testify/require"
)

// mockConsumerGroupSession implements sarama.ConsumerGroupSession
type mockConsumerGroupSession struct {
	ctx context.Context
}

func newMockConsumerGroupSession() *mockConsumerGroupSession {
	return &mockConsumerGroupSession{
		ctx: context.Background(),
	}
}

func (m *mockConsumerGroupSession) Claims() map[string][]int32 { return nil }
func (m *mockConsumerGroupSession) MemberID() string           { return "mock-member" }
func (m *mockConsumerGroupSession) GenerationID() int32        { return 1 }
func (m *mockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
}
func (m *mockConsumerGroupSession) Commit()                                   {}
func (m *mockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
}
func (m *mockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {}
func (m *mockConsumerGroupSession) Context() context.Context                                 { return m.ctx }

// mockConsumerGroupClaim implements sarama.ConsumerGroupClaim
type mockConsumerGroupClaim struct {
	messages chan *sarama.ConsumerMessage
	ctx      context.Context
	cancel   context.CancelFunc
}

func newMockConsumerGroupClaim() *mockConsumerGroupClaim {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockConsumerGroupClaim{
		messages: make(chan *sarama.ConsumerMessage),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (m *mockConsumerGroupClaim) Topic() string                            { return "test-topic" }
func (m *mockConsumerGroupClaim) Partition() int32                         { return 0 }
func (m *mockConsumerGroupClaim) InitialOffset() int64                     { return 0 }
func (m *mockConsumerGroupClaim) HighWaterMarkOffset() int64               { return 0 }
func (m *mockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage { return m.messages }

func (m *mockConsumerGroupClaim) Close() {
	m.cancel()
	close(m.messages)
}

// TestReadyChannelRaceCondition verifies that multiple ConsumeClaim calls
// (which can happen during rebalances) don't cause issues with the ready channel
func TestReadyChannelRaceCondition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processor := newMessageProcessor(ctx)

	// Simulate multiple calls to ConsumeClaim (as happens during rebalances)
	// This should not deadlock or panic
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session := newMockConsumerGroupSession()
			claim := newMockConsumerGroupClaim()
			defer claim.Close()

			// This should return quickly when the claim is closed
			err := processor.ConsumeClaim(session, claim)
			require.NoError(t, err)
		}()
	}

	// Should receive exactly one ready signal
	select {
	case <-processor.ready:
		// Good - received the ready signal
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive ready signal within timeout")
	}

	// There should be no more ready signals
	select {
	case <-processor.ready:
		t.Fatal("Received unexpected second ready signal")
	case <-time.After(100 * time.Millisecond):
		// Good - no additional signals
	}

	cancel()
	wg.Wait()
}

// TestConnectorReadyChannelRaceCondition verifies the same for the connector
func TestConnectorReadyChannelRaceCondition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	translator := func(m *sarama.ConsumerMessage) *workflow.ConnectorEvent {
		return &workflow.ConnectorEvent{
			ID:        "test",
			ForeignID: string(m.Key),
		}
	}
	processor := newConnectorProcessor(ctx, translator)

	// Simulate multiple calls to ConsumeClaim
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session := newMockConsumerGroupSession()
			claim := newMockConsumerGroupClaim()
			defer claim.Close()

			err := processor.ConsumeClaim(session, claim)
			require.NoError(t, err)
		}()
	}

	// Should receive exactly one ready signal
	select {
	case <-processor.ready:
		// Good - received the ready signal
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive ready signal within timeout")
	}

	// There should be no more ready signals
	select {
	case <-processor.ready:
		t.Fatal("Received unexpected second ready signal")
	case <-time.After(100 * time.Millisecond):
		// Good - no additional signals
	}

	cancel()
	wg.Wait()
}

// TestIteratorChannelBuffering verifies that the iterator channel has proper buffering
// to prevent backpressure issues
func TestIteratorChannelBuffering(t *testing.T) {
	ctx := context.Background()

	processor := newMessageProcessor(ctx)

	// Verify the iterator channel has buffering
	require.Greater(t, cap(processor.iterator), 0,
		"Iterator channel should be buffered to prevent backpressure")

	// Verify we can send multiple items without blocking (up to buffer size)
	session := newMockConsumerGroupSession()
	testMsg := &sarama.ConsumerMessage{
		Key:    []byte("test"),
		Value:  []byte("1"),
		Offset: 0,
	}

	// Should be able to buffer at least a few messages
	for i := 0; i < 10; i++ {
		select {
		case processor.iterator <- consume(session, testMsg):
			// Good - sent without blocking
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Channel blocked after only %d messages (buffer size: %d)", i, cap(processor.iterator))
		}
	}
}

// TestConnectorIteratorChannelBuffering verifies the same for connector
func TestConnectorIteratorChannelBuffering(t *testing.T) {
	ctx := context.Background()

	translator := func(m *sarama.ConsumerMessage) *workflow.ConnectorEvent {
		return &workflow.ConnectorEvent{
			ID:        "test",
			ForeignID: string(m.Key),
		}
	}
	processor := newConnectorProcessor(ctx, translator)

	// Verify the iterator channel has buffering
	require.Greater(t, cap(processor.iterator), 0,
		"Iterator channel should be buffered to prevent backpressure")

	// Verify we can send multiple items without blocking (up to buffer size)
	session := newMockConsumerGroupSession()
	testMsg := &sarama.ConsumerMessage{
		Key:    []byte("test"),
		Value:  []byte("test"),
		Offset: 0,
	}

	// Should be able to buffer at least a few messages
	for i := 0; i < 10; i++ {
		select {
		case processor.iterator <- consumeConnectorEvent(session, testMsg, translator):
			// Good - sent without blocking
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Channel blocked after only %d messages (buffer size: %d)", i, cap(processor.iterator))
		}
	}
}

// TestEventIDUsesOffset verifies that Event.ID is set to the offset directly
func TestEventIDUsesOffset(t *testing.T) {
	session := newMockConsumerGroupSession()

	testCases := []struct {
		name   string
		offset int64
	}{
		{"offset 0", 0},
		{"offset 100", 100},
		{"offset 999", 999},
		{"large offset", 123456789},
		// Note: Kafka offsets are typically non-negative in production, but we test
		// negative offsets to ensure the code doesn't transform or validate the value.
		// The Event.ID should exactly match whatever offset Kafka provides.
		{"negative offset", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &sarama.ConsumerMessage{
				Key:    []byte("test-key"),
				Value:  []byte("1"),
				Offset: tc.offset,
			}

			consumeFn := consume(session, msg)
			event, _, err := consumeFn()
			require.NoError(t, err)

			// Verify ID is exactly the offset, not offset+1
			require.Equal(t, tc.offset, event.ID,
				"Event ID should equal the Kafka offset directly")
		})
	}
}
