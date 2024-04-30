package kafkastreamer

import (
	"context"

	"github.com/segmentio/kafka-go"

	"github.com/luno/workflow"
)

func NewConnector(c kafka.ReaderConfig, t Translator) *connector {
	return &connector{
		translator: t,
		config:     c,
	}
}

type Translator func(m kafka.Message) workflow.ConnectorEvent

type connector struct {
	translator Translator
	config     kafka.ReaderConfig
}

func (c *connector) Make(ctx context.Context, name string) (workflow.ConnectorConsumer, error) {
	c.config.GroupID = name

	kafkaReader := kafka.NewReader(c.config)
	return &consumer{
		name:       name,
		translator: c.translator,
		reader:     kafkaReader,
	}, nil
}

type consumer struct {
	name       string
	translator Translator
	reader     *kafka.Reader
}

func (c *consumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	var commit []kafka.Message
	for ctx.Err() == nil {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return nil, nil, err
		}

		// Append the message to the commit slice to ensure we send all messages that have been processed
		commit = append(commit, m)

		e := c.translator(m)
		return &e, func() error {
			return c.reader.CommitMessages(ctx, commit...)
		}, nil
	}

	return nil, nil, ctx.Err()
}

func (c *consumer) Close() error {
	return c.reader.Close()
}
