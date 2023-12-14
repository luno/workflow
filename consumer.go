package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/log"
)

// ConsumerFunc provides a record that is expected to be modified if the data needs to change. If true is returned with
// a nil error then the record, along with its modifications, will be stored. If false is returned with a nil error then
// the record will not be stored and the event will be skipped and move onto the next event. If a non-nil error is
// returned then the consumer will back off and try again until a nil error occurs or the retry max has been reached
// if a Dead Letter Queue has been configured for the workflow.
type ConsumerFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status]) (bool, error)

type consumerConfig[Type any, Status StatusType] struct {
	PollingFrequency  time.Duration
	ErrBackOff        time.Duration
	DestinationStatus Status
	Consumer          ConsumerFunc[Type, Status]
	ParallelCount     int
}

func consumer[Type any, Status StatusType](w *Workflow[Type, Status], currentStatus Status, p consumerConfig[Type, Status], shard, totalShards int) {
	role := makeRole(
		w.Name,
		fmt.Sprintf("%v", int(currentStatus)),
		"to",
		fmt.Sprintf("%v", int(p.DestinationStatus)),
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	w.run(role, func(ctx context.Context) error {
		err := runStepConsumerForever[Type, Status](ctx, w, p, currentStatus, role, shard, totalShards)
		if errors.IsAny(err, ErrWorkflowShutdown, context.Canceled) {
			if w.debugMode {
				log.Info(ctx, "shutting down consumer", j.MKV{
					"workflow_name":      w.Name,
					"current_status":     currentStatus,
					"destination_status": p.DestinationStatus,
				})
			}
		} else if err != nil {
			log.Error(ctx, errors.Wrap(err, "consumer error"))
		}

		return nil
	}, p.ErrBackOff)
}

func runStepConsumerForever[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], p consumerConfig[Type, Status], status Status, role string, shard, totalShards int) error {
	pollFrequency := w.defaultPollingFrequency
	if p.PollingFrequency.Nanoseconds() != 0 {
		pollFrequency = p.PollingFrequency
	}

	topic := Topic(w.Name, int(status))
	stream := w.eventStreamerFn.NewConsumer(
		topic,
		role,
		WithConsumerPollFrequency(pollFrequency),
		WithEventFilter(
			shardFilter(shard, totalShards),
		),
	)

	defer stream.Close()

	for {
		if ctx.Err() != nil {
			return errors.Wrap(ErrWorkflowShutdown, "")
		}

		e, ack, err := stream.Recv(ctx)
		if err != nil {
			return err
		}

		if e.Headers[HeaderWorkflowName] != w.Name {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		if e.Type != int(status) {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		record, err := w.recordStore.Lookup(ctx, e.ForeignID)
		if errors.Is(err, ErrRecordNotFound) {
			err = ack()
			if err != nil {
				return err
			}

			continue
		} else if err != nil {
			return err
		}

		// Check to see if record is in expected state. If the status isn't in the expected state then skip for
		// idempotency.
		if record.Status != e.Type {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		err = consume(ctx, record, p.Consumer, ack, p.DestinationStatus, w.endPoints, w.eventStreamerFn, w.recordStore)
		if err != nil {
			return err
		}
	}
}

func shardFilter(shard, totalShards int) EventFilter {
	return func(e *Event) bool {
		if totalShards > 1 {
			return e.ID%int64(totalShards) == int64(shard)
		}

		return false
	}
}

func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}

	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		return errors.Wrap(ErrWorkflowShutdown, ctx.Err().Error())
	case <-t.C:
		return nil
	}
}

func consume[Type any, Status StatusType](ctx context.Context, wr *WireRecord, cf ConsumerFunc[Type, Status], ack Ack, destinationStatus Status, endPoints map[Status]bool, es EventStreamer, rs RecordStore) error {
	var t Type
	err := Unmarshal(wr.Object, &t)
	if err != nil {
		return err
	}

	record := Record[Type, Status]{
		WireRecord: *wr,
		Status:     Status(wr.Status),
		Object:     &t,
	}

	ok, err := cf(ctx, &record)
	if err != nil {
		return errors.Wrap(err, "failed to consume", j.MKV{
			"workflow_name":      wr.WorkflowName,
			"foreign_id":         wr.ForeignID,
			"current_status":     wr.Status,
			"destination_status": destinationStatus,
		})
	}

	if ok {
		b, err := Marshal(&record.Object)
		if err != nil {
			return err
		}

		isEnd := endPoints[destinationStatus]
		wr := &WireRecord{
			ID:           record.ID,
			RunID:        record.RunID,
			WorkflowName: record.WorkflowName,
			ForeignID:    record.ForeignID,
			Status:       int(destinationStatus),
			IsStart:      false,
			IsEnd:        isEnd,
			Object:       b,
			CreatedAt:    record.CreatedAt,
		}

		err = update(ctx, es, rs, wr)
		if err != nil {
			return err
		}
	}

	return ack()
}
