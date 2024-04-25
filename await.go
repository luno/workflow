package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
)

func (w *Workflow[Type, Status]) Await(ctx context.Context, foreignID, runID string, status Status, opts ...AwaitOption) (*Record[Type, Status], error) {
	var opt awaitOpts
	for _, option := range opts {
		option(&opt)
	}

	pollFrequency := w.defaultPollingFrequency
	if opt.pollFrequency.Nanoseconds() != 0 {
		pollFrequency = opt.pollFrequency
	}

	role := makeRole("await", w.Name, fmt.Sprintf("%v", int(status)), foreignID)
	return awaitWorkflowStatusByForeignID[Type, Status](ctx, w, status, foreignID, runID, role, pollFrequency)
}

func awaitWorkflowStatusByForeignID[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], status Status, foreignID, runID string, role string, pollFrequency time.Duration) (*Record[Type, Status], error) {
	topic := Topic(w.Name, int(status))
	stream, err := w.eventStreamer.NewConsumer(
		ctx,
		topic,
		role,
		WithConsumerPollFrequency(pollFrequency),
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

		shouldFilter := FilterUsing(e,
			FilterByForeignID(foreignID),
			FilterByRunID(runID),
			runStateUpdatesFilter(),
		)
		if shouldFilter {
			err = ack()
			if err != nil {
				return nil, err
			}

			continue
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
			controller: newRunStateController(r, w.recordStore.Store, w.customDelete),
		}, ack()
	}
}

type awaitOpts struct {
	pollFrequency time.Duration
}

type AwaitOption func(o *awaitOpts)

func WithPollingFrequency(d time.Duration) AwaitOption {
	return func(o *awaitOpts) {
		o.pollFrequency = d
	}
}
