package memstreamer

import (
	"context"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/andrewwormald/workflow"
)

func New(opts ...Option) *StreamConstructor {
	var (
		log []*workflow.Event
		opt options
	)

	// Set a default clock
	opt.clock = clock.RealClock{}

	for _, option := range opts {
		option(&opt)
	}

	return &StreamConstructor{
		opts: &opt,
		stream: &Stream{
			mu:  &sync.Mutex{},
			log: &log,
		},
	}
}

type options struct {
	clock clock.Clock
}

type Option func(o *options)

func WithClock(clock clock.Clock) Option {
	return func(o *options) {
		o.clock = clock
	}
}

type StreamConstructor struct {
	opts   *options
	stream *Stream
}

func (s StreamConstructor) NewProducer(topic string) workflow.Producer {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	return &Stream{
		mu:    s.stream.mu,
		log:   s.stream.log,
		topic: topic,
		clock: s.opts.clock,
	}
}

func (s StreamConstructor) NewConsumer(topic string, name string, opts ...workflow.ConsumerOption) workflow.Consumer {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	return &Stream{
		mu:    s.stream.mu,
		log:   s.stream.log,
		topic: topic,
		name:  name,
		clock: s.opts.clock,
	}
}

var _ workflow.EventStreamer = (*StreamConstructor)(nil)

type Stream struct {
	mu     *sync.Mutex
	log    *[]*workflow.Event
	offset int
	topic  string
	name   string
	clock  clock.Clock
}

func (s *Stream) Send(ctx context.Context, recordID int64, statusType int, headers map[workflow.Header]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	length := len(*s.log)
	*s.log = append(*s.log, &workflow.Event{
		ID:        int64(length) + 1,
		ForeignID: recordID,
		Type:      statusType,
		Headers:   headers,
		CreatedAt: s.clock.Now(),
	})

	return nil
}

func (s *Stream) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	for ctx.Err() == nil {
		s.mu.Lock()
		log := *s.log
		s.mu.Unlock()

		if len(log)-1 < s.offset {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		e := log[s.offset]

		if s.topic != e.Headers[workflow.HeaderTopic] {
			s.offset += 1
			continue
		}

		return e, func() error {
			s.offset += 1
			return nil
		}, nil
	}

	return nil, nil, ctx.Err()
}

func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log = nil
	s.offset = 0
	return nil
}

var (
	_ workflow.Producer = (*Stream)(nil)
	_ workflow.Consumer = (*Stream)(nil)
)
