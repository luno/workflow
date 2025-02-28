package kafkastreamer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/luno/workflow"
)

func NewConnector(brokers []string, c *sarama.Config, t Translator, topic string) *connector {
	return &connector{
		brokers:    brokers,
		config:     c,
		translator: t,
		topic:      topic,
	}
}

type Translator func(m *sarama.ConsumerMessage) *workflow.ConnectorEvent

type connector struct {
	brokers    []string
	config     *sarama.Config
	translator Translator
	topic      string
}

func (c *connector) Make(ctx context.Context, name string) (workflow.ConnectorConsumer, error) {
	c.config.Consumer.Offsets.Initial = sarama.OffsetOldest
	cg, err := sarama.NewConsumerGroup(c.brokers, name, c.config)
	if err != nil {
		return nil, err
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	processor := newConnectorProcessor(consumeCtx, c.translator)
	go func() {
		for ctx.Err() == nil {
			err := cg.Consume(consumeCtx, []string{c.topic}, processor)
			if err != nil && errors.Is(err, context.Canceled) {
				// Exit on context cancellation
				return
			} else if err != nil {
				err = fmt.Errorf("kafka consumer exited unexpectedly: %w", err)
				fmt.Println(err)
				time.Sleep(time.Second)
				continue
			}

			time.Sleep(time.Millisecond * 250)
		}
	}()

	// Wait for the processor to be ready
	<-processor.ready

	return &consumer{
		name:               name,
		cancel:             cancel,
		translator:         c.translator,
		connectorProcessor: processor,
	}, nil
}

type consumer struct {
	name               string
	cancel             context.CancelFunc
	translator         Translator
	connectorProcessor *connectorProcessor
}

func (c *consumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case next := <-c.connectorProcessor.iterator:
			return next()
		}
	}

	return nil, nil, ctx.Err()
}

func (c *consumer) Close() error {
	c.cancel()
	return nil
}

func newConnectorProcessor(ctx context.Context, translator Translator) *connectorProcessor {
	return &connectorProcessor{
		ctx:        ctx,
		ready:      make(chan bool),
		translator: translator,
		iterator:   make(chan func() (*workflow.ConnectorEvent, workflow.Ack, error)),
	}
}

// connectorProcessor implements the sarama.ConsumerGroupHandler interface
type connectorProcessor struct {
	ctx        context.Context
	ready      chan bool
	translator Translator
	iterator   chan func() (*workflow.ConnectorEvent, workflow.Ack, error)
}

func (cp *connectorProcessor) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (cp *connectorProcessor) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from Kafka
func (cp *connectorProcessor) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	cp.ready <- true
	for {
		select {
		case m := <-claim.Messages():
			cp.iterator <- func() (*workflow.ConnectorEvent, workflow.Ack, error) {
				return cp.translator(m), func() error {
					session.MarkMessage(m, "")
					return nil
				}, nil
			}
		case <-cp.ctx.Done():
			return nil
		case <-session.Context().Done():
			return nil
		}
	}
}
