package kafkastreamer

import (
	"testing"

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
