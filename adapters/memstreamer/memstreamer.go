package memstreamer

import (
	"context"
	"sync"

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

	mu := &sync.Mutex{}
	return &StreamConstructor{
		opts: &opt,
		stream: &Stream{
			mu:   mu,
			cond: sync.NewCond(mu),
			log:  &log,
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

func (s StreamConstructor) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	return &Stream{
		mu:    s.stream.mu,
		cond:  s.stream.cond,
		log:   s.stream.log,
		topic: topic,
		clock: s.opts.clock,
	}, nil
}

func (s StreamConstructor) NewReceiver(
	ctx context.Context,
	topic string,
	name string,
	opts ...workflow.ReceiverOption,
) (workflow.EventReceiver, error) {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()

	var options workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&options)
	}

	// Set the cursor to the current log length at receiver creation time so
	// that events sent after NewReceiver but before the first Recv are still
	// received.
	if options.StreamFromLatest && s.cursorStore.Get(name) == 0 {
		s.cursorStore.Set(name, len(*s.stream.log))
	}

	return &Stream{
		mu:          s.stream.mu,
		cond:        s.stream.cond,
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
	cond        *sync.Cond
	log         *[]*workflow.Event
	cursorStore *cursorStore
	topic       string
	name        string
	clock       clock.Clock
	options     workflow.ReceiverOptions
}

func (s *Stream) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	length := len(*s.log)
	*s.log = append(*s.log, &workflow.Event{
		ID:        int64(length) + 1,
		ForeignID: foreignID,
		Type:      statusType,
		Headers:   headers,
		CreatedAt: s.clock.Now(),
	})

	// Wake any receivers parked on cond.Wait so they can pick up the new event.
	s.cond.Broadcast()

	return nil
}

func (s *Stream) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	// sync.Cond doesn't natively respect ctx. Spawn a watcher that
	// broadcasts when the ctx is done so Recv can wake up and return
	// ctx.Err() instead of leaking the goroutine forever. The stop channel
	// is closed on return so the watcher exits when Recv exits (no leak per
	// call).
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.cond.Broadcast()
			s.mu.Unlock()
		case <-stop:
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		log := *s.log
		cursorOffset := s.cursorStore.Get(s.name)

		if len(log)-1 < cursorOffset {
			// Nothing to deliver — park until Send or ctx cancel signals us.
			s.cond.Wait()
			continue
		}

		e := log[cursorOffset]

		// Skip events that are not related to this topic. Advance the cursor
		// past the skipped event so we don't see it again.
		if s.topic != e.Headers[workflow.HeaderTopic] {
			s.cursorStore.Set(s.name, cursorOffset+1)
			continue
		}

		// The ack closure captures cursorOffset so a double-call is a no-op
		// (it would set the cursor to the same value). The cursorStore's
		// own mutex protects the cursor map; that's separate from s.mu so
		// we don't need to hold s.mu here.
		return e, func() error {
			s.cursorStore.Set(s.name, cursorOffset+1)
			return nil
		}, nil
	}
}

func (s *Stream) Close() error {
	return nil
}

var (
	_ workflow.EventSender   = (*Stream)(nil)
	_ workflow.EventReceiver = (*Stream)(nil)
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
