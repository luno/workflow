package memstreamer

import (
	"context"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
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
		cursorStore: newCursorStore(),
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
	opts        *options
	stream      *Stream
	cursorStore *cursorStore
}

func (s StreamConstructor) NewProducer(ctx context.Context, topic string) (workflow.Producer, error) {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	return &Stream{
		mu:    s.stream.mu,
		log:   s.stream.log,
		topic: topic,
		clock: s.opts.clock,
	}, nil
}

func (s StreamConstructor) NewConsumer(ctx context.Context, topic string, name string, opts ...workflow.ConsumerOption) (workflow.Consumer, error) {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	var options workflow.ConsumerOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &Stream{
		mu:          s.stream.mu,
		log:         s.stream.log,
		cursorStore: s.cursorStore,
		topic:       topic,
		name:        name,
		clock:       s.opts.clock,
		options:     options,
	}, nil
}

var _ workflow.EventStreamer = (*StreamConstructor)(nil)

type Stream struct {
	mu          *sync.Mutex
	log         *[]*workflow.Event
	cursorStore *cursorStore
	topic       string
	name        string
	clock       clock.Clock
	options     workflow.ConsumerOptions
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

		cursorOffset := s.cursorStore.Get(s.name)
		if len(log)-1 < cursorOffset {
			time.Sleep(time.Millisecond * 10)
			s.mu.Unlock()
			continue
		}

		e := log[cursorOffset]

		// Skip events that are not related to this topic
		if s.topic != e.Headers[workflow.HeaderTopic] {
			s.cursorStore.Set(s.name, cursorOffset+1)
			s.mu.Unlock()
			continue
		}

		// Filter out unwanted events
		filter := s.options.EventFilter
		if filter != nil {
			if skip := filter(e); skip {
				s.cursorStore.Set(s.name, cursorOffset+1)
				s.mu.Unlock()
				continue
			}
		}

		s.mu.Unlock()
		return e, func() error {
			s.cursorStore.Set(s.name, cursorOffset+1)
			return nil
		}, nil
	}

	return nil, nil, ctx.Err()
}

func (s *Stream) Close() error {
	return nil
}

var (
	_ workflow.Producer = (*Stream)(nil)
	_ workflow.Consumer = (*Stream)(nil)
)

func newCursorStore() *cursorStore {
	return &cursorStore{
		cursors: make(map[string]int),
	}
}

type cursorStore struct {
	mu      sync.Mutex
	cursors map[string]int
}

func (cs *cursorStore) Get(name string) int {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	return cs.cursors[name]
}

func (cs *cursorStore) Set(name string, value int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.cursors[name] = value
}
