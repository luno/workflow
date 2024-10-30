package workflow

import (
	"context"
	"time"
)

// EventStreamer implementations should all be tested with adaptertest.TestEventStreamer
type EventStreamer interface {
	NewProducer(ctx context.Context, topic string) (Producer, error)
	NewConsumer(ctx context.Context, topic string, name string, opts ...ConsumerOption) (Consumer, error)
}

type Producer interface {
	Send(ctx context.Context, foreignID string, statusType int, headers map[Header]string) error
	Close() error
}

type Consumer interface {
	Recv(ctx context.Context) (*Event, Ack, error)
	Close() error
}

// Ack is used for the event streamer to safeUpdate its cursor of what messages have
// been consumed. If Ack is not called then the event streamer, depending on implementation,
// will likely not keep track of which records / events have been consumed.
type Ack func() error

type ConsumerOptions struct {
	PollFrequency time.Duration
	Lag           time.Duration
}

type ConsumerOption func(*ConsumerOptions)

func WithConsumerPollFrequency(d time.Duration) ConsumerOption {
	return func(opt *ConsumerOptions) {
		opt.PollFrequency = d
	}
}
