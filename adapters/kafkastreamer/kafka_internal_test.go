package kafkastreamer

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
)

func TestWithConfig(t *testing.T) {
	brokers := []string{"localhost:9092"}
	customConfig := sarama.NewConfig()
	customConfig.Producer.MaxMessageBytes = 2000000
	customConfig.Consumer.Fetch.Default = 2048576

	sc := New(brokers, WithConfig(customConfig))

	// Since this is an internal test, we can access private fields
	require.Equal(t, 2000000, sc.sharedConfig.Producer.MaxMessageBytes)
	require.Equal(t, int32(2048576), sc.sharedConfig.Consumer.Fetch.Default)
}

func TestWithConfigOverridesDefaults(t *testing.T) {
	brokers := []string{"localhost:9092"}

	// First verify our default config
	defaultSC := New(brokers)
	require.True(t, defaultSC.sharedConfig.Producer.Return.Successes)
	require.True(t, defaultSC.sharedConfig.Producer.Return.Errors)

	// Now test that custom config overrides defaults
	customConfig := sarama.NewConfig()
	customConfig.Producer.Return.Successes = false
	customConfig.Producer.Return.Errors = false

	customSC := New(brokers, WithConfig(customConfig))

	// Verify that the custom config completely replaced the default
	require.False(t, customSC.sharedConfig.Producer.Return.Successes)
	require.False(t, customSC.sharedConfig.Producer.Return.Errors)
}

func TestDefaultConfig(t *testing.T) {
	brokers := []string{"localhost:9092"}
	sc := New(brokers)

	// Verify default config values
	require.True(t, sc.sharedConfig.Producer.Return.Successes)
	require.True(t, sc.sharedConfig.Producer.Return.Errors)
}

func TestPanicIfConfigNil(t *testing.T) {
	require.PanicsWithValue(t,
		"sarama config cannot be nil",
		func() {
			_ = New([]string{""}, WithConfig(nil))
		}, "")
}

func TestWait_ZeroDuration(t *testing.T) {
	err := wait(context.Background(), 0)
	require.NoError(t, err)
}

func TestWait_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := wait(ctx, time.Hour)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWait_CompletesAfterDuration(t *testing.T) {
	ctx := context.Background()
	start := time.Now()

	err := wait(ctx, 10*time.Millisecond)

	require.NoError(t, err)
	require.True(t, time.Since(start) >= 10*time.Millisecond)
}

func TestSend_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sender := &Sender{
		Topic:         "test-topic",
		Writer:        nil, // Won't be reached since ctx is already cancelled
		WriterTimeout: time.Second,
	}

	err := sender.Send(ctx, "foreign-id", 1, nil)
	require.ErrorIs(t, err, context.Canceled)
}
