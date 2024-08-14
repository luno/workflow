package kafkastreamer_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/segmentio/kafka-go"

	"github.com/luno/workflow/adapters/kafkastreamer"
)

const brokerAddress = "localhost:9092"

func TestStreamer(t *testing.T) {
	constructor := kafkastreamer.New([]string{brokerAddress})
	adaptertest.RunEventStreamerTest(t, constructor)
}

func TestConnector(t *testing.T) {
	config := kafka.ReaderConfig{
		Brokers:        []string{brokerAddress},
		Topic:          "test-connector-topic",
		ReadBackoffMin: time.Millisecond * 100,
		ReadBackoffMax: time.Second,
		StartOffset:    kafka.FirstOffset,
		QueueCapacity:  1000,
		MinBytes:       10,  // 10B
		MaxBytes:       1e9, // 9MB
		MaxWait:        time.Second,
	}
	translator := func(m kafka.Message) workflow.ConnectorEvent {
		return workflow.ConnectorEvent{
			ID:        strconv.FormatInt(m.Offset, 10),
			ForeignID: string(m.Key),
			Type:      m.Topic,
			CreatedAt: m.Time,
		}
	}
	constructor := kafkastreamer.NewConnector(config, translator)
	adaptertest.RunConnectorTest(t, func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor {
		writer := &kafka.Writer{
			Addr:                   kafka.TCP(brokerAddress),
			Topic:                  "test-connector-topic",
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireOne,
		}
		defer writer.Close()

		ctx := context.Background()
		for _, e := range seedEvents {

			m := kafka.Message{
				Key: []byte(e.ForeignID),
			}
			err := writer.WriteMessages(ctx, m)
			jtest.RequireNil(t, err)
		}

		return constructor
	})
}
