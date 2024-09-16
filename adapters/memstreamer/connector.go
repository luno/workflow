package memstreamer

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
)

func NewConnector(events []workflow.ConnectorEvent, opts ...Option) *connector {
	var log []*workflow.ConnectorEvent
	for _, event := range events {
		e := event
		log = append(log, &e)
	}

	var opt options
	opt.clock = clock.RealClock{}

	for _, option := range opts {
		option(&opt)
	}

	return &connector{
		log:         &log,
		opts:        &opt,
		cursorStore: newCursorStore(),
	}
}

type connector struct {
	log         *[]*workflow.ConnectorEvent
	opts        *options
	cursorStore *cursorStore
}

func (c *connector) Make(ctx context.Context, name string) (workflow.ConnectorConsumer, error) {
	return &consumer{
		log:         c.log,
		cursorStore: c.cursorStore,
		cursorName:  name,
		clock:       c.opts.clock,
	}, nil
}

type consumer struct {
	mu          sync.Mutex
	log         *[]*workflow.ConnectorEvent
	cursorStore *cursorStore
	cursorName  string
	clock       clock.Clock
	options     workflow.ConsumerOptions
}

func (c *consumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
	for ctx.Err() == nil {
		cursorOffset := c.cursorStore.Get(c.cursorName)
		event, err := c.next()
		if errors.Is(err, errReachedHeadOfStream) {
			// NoReturnErr: Sleep and continue until a new event is added
			time.Sleep(time.Millisecond * 10)
			continue
		} else if err != nil {
			return nil, nil, err
		}

		return event, func() error {
			c.cursorStore.Set(c.cursorName, cursorOffset+1)
			return nil
		}, nil
	}

	return nil, nil, ctx.Err()
}

var errReachedHeadOfStream = errors.New("reached head of stream")

func (c *consumer) next() (*workflow.ConnectorEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	log := *c.log

	cursorOffset := c.cursorStore.Get(c.cursorName)
	if len(log)-1 < cursorOffset {
		return nil, errReachedHeadOfStream
	}

	return log[cursorOffset], nil
}

func (c *consumer) Close() error {
	return nil
}
