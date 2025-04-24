package kafkastreamer_test

import (
	"strconv"
	"testing"

	"github.com/IBM/sarama"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkacontainer "github.com/testcontainers/testcontainers-go/modules/kafka"

	"github.com/luno/workflow/adapters/kafkastreamer"
)

const brokerAddress = "localhost:9092"

func TestStreamer(t *testing.T) {
	adaptertest.RunEventStreamerTest(t, func() workflow.EventStreamer {
		ctx := t.Context()

		kafkaInstance, err := kafkacontainer.Run(ctx, "confluentinc/confluent-local:7.5.0", kafkacontainer.WithClusterID("kraftCluster"))
		testcontainers.CleanupContainer(t, kafkaInstance)
		require.NoError(t, err)

		brokers, err := kafkaInstance.Brokers(ctx)
		require.NoError(t, err)

		return kafkastreamer.New(brokers)
	})
}

func TestConnector(t *testing.T) {
	ctx := t.Context()
	topic := "connector-topic"
	kafkaInstance, err := kafkacontainer.Run(ctx, "confluentinc/confluent-local:7.5.0", kafkacontainer.WithClusterID("kraftCluster"))
	testcontainers.CleanupContainer(t, kafkaInstance)
	require.NoError(t, err)

	brokers, err := kafkaInstance.Brokers(ctx)
	require.NoError(t, err)

	translator := func(m *sarama.ConsumerMessage) *workflow.ConnectorEvent {
		return &workflow.ConnectorEvent{
			ID:        strconv.FormatInt(m.Offset, 10),
			ForeignID: string(m.Key),
			Type:      m.Topic,
			CreatedAt: m.Timestamp,
		}
	}
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	constructor := kafkastreamer.NewConnector(brokers, config, translator, topic)
	adaptertest.RunConnectorTest(t, func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor {
		producer, err := sarama.NewSyncProducer(brokers, config)
		require.NoError(t, err)

		for _, e := range seedEvents {
			m := &sarama.ProducerMessage{
				Topic: topic,
				Key:   sarama.StringEncoder(e.ForeignID),
			}

			_, _, err := producer.SendMessage(m)
			require.NoError(t, err)
		}

		return constructor
	})
}
