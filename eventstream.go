package workflow

import (
	"context"
	"strconv"
	"time"

	"github.com/luno/jettison/errors"
)

// EventStreamer implementations should all be tested with adaptertest.TestEventStreamer
type EventStreamer interface {
	NewProducer(ctx context.Context, topic string) (Producer, error)
	NewConsumer(ctx context.Context, topic string, name string, opts ...ConsumerOption) (Consumer, error)
}

type Producer interface {
	Send(ctx context.Context, recordID int64, statusType int, headers map[Header]string) error
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
	EventFilter   EventFilter
	Lag           time.Duration
}

// EventFilter can be passed to the event streaming implementation to allow specific consumers to have an
// earlier on filtering process. True is returned when the event should be skipped.
type EventFilter func(e *Event) bool

type ConsumerOption func(*ConsumerOptions)

func WithEventFilters(ef ...EventFilter) ConsumerOption {
	return func(opt *ConsumerOptions) {
		opt.EventFilter = func(e *Event) bool {
			for _, filter := range ef {
				if mustFilterOut := filter(e); mustFilterOut {
					return true
				}
			}

			return false
		}
	}
}

func shardFilter(shard, totalShards int) EventFilter {
	return func(e *Event) bool {
		if totalShards > 1 {
			return e.ID%int64(totalShards) != int64(shard)-1
		}

		return false
	}
}

func runStateUpdatesFilter() EventFilter {
	return func(e *Event) bool {
		if e.Headers[HeaderPreviousRunState] == "" || e.Headers[HeaderRunState] == "" {
			return false
		}

		intValue, err := strconv.ParseInt(e.Headers[HeaderPreviousRunState], 10, 64)
		if err != nil {
			// NoReturnErr: Ignore failure to parse int from string
			return false
		}

		previous := RunState(intValue)

		intValue, err = strconv.ParseInt(e.Headers[HeaderRunState], 10, 64)
		if err != nil {
			// NoReturnErr: Ignore failure to parse int from string
			return false
		}

		current := RunState(intValue)

		if current.Stopped() {
			return true
		}

		if previous == RunStateInitiated && current == RunStateRunning {
			// Ignore all events generated from moving from Initiated to Running
			return true
		}

		return false
	}
}

func FilterByWorkflowName(workflowName string) EventFilter {
	return func(e *Event) bool {
		return e.Headers[HeaderWorkflowName] != workflowName
	}
}

func FilterByForeignID(foreignID string) EventFilter {
	return func(e *Event) bool {
		fid, ok := e.Headers[HeaderWorkflowForeignID]
		if !ok {
			return false
		}

		return fid != foreignID
	}
}

func FilterByRunID(runID string) EventFilter {
	return func(e *Event) bool {
		rID, ok := e.Headers[HeaderRunID]
		if !ok {
			return false
		}

		return rID != runID
	}
}

func FilterByStatus(status int) EventFilter {
	return func(e *Event) bool {
		return e.Type != status
	}
}

func WithConsumerPollFrequency(d time.Duration) ConsumerOption {
	return func(opt *ConsumerOptions) {
		opt.PollFrequency = d
	}
}

func awaitWorkflowStatusByForeignID[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, foreignID, runID string, role string, pollFrequency time.Duration) (*Record[Type, Status], error) {
	topic := Topic(w.Name, int(status))
	stream, err := w.eventStreamer.NewConsumer(
		ctx,
		topic,
		role,
		WithConsumerPollFrequency(pollFrequency),
		WithEventFilters(
			FilterByForeignID(foreignID),
			FilterByRunID(runID),
			runStateUpdatesFilter(),
		),
	)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		e, ack, err := stream.Recv(ctx)
		if err != nil {
			return nil, err
		}

		r, err := w.recordStore.Lookup(ctx, e.ForeignID)
		if errors.Is(err, ErrRecordNotFound) {
			err = ack()
			if err != nil {
				return nil, err
			}

			continue
		} else if err != nil {
			return nil, err
		}

		var t Type
		err = Unmarshal(r.Object, &t)
		if err != nil {
			return nil, err
		}

		return &Record[Type, Status]{
			WireRecord: *r,
			Status:     Status(r.Status),
			Object:     &t,
			stopper:    newRunStateController[Status](r, w.recordStore, storeAndEmit),
		}, ack()
	}
}
