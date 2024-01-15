package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"

	"github.com/andrewwormald/workflow/internal/metrics"
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
	LagAlert          time.Duration
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

	pollFrequency := w.defaultPollingFrequency
	if p.PollingFrequency.Nanoseconds() != 0 {
		pollFrequency = p.PollingFrequency
	}

	topic := Topic(w.Name, int(currentStatus))
	stream := w.eventStreamerFn.NewConsumer(
		topic,
		role,
		WithConsumerPollFrequency(pollFrequency),
		WithEventFilter(
			shardFilter(shard, totalShards),
		),
	)

	defer stream.Close()

	// processName can change in value if the string value of the status enum is changed. It should not be used for
	// storing in the record store, event streamer, timeoutstore, or offset store.
	processName := makeRole(
		fmt.Sprintf("%v", currentStatus.String()),
		"to",
		fmt.Sprintf("%v", p.DestinationStatus.String()),
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	w.run(role, processName, func(ctx context.Context) error {
		return consumeForever[Type, Status](ctx, w, p, stream, currentStatus, processName)
	}, p.ErrBackOff)
}

func consumeForever[Type any, Status StatusType](ctx context.Context, w *Workflow[Type, Status], p consumerConfig[Type, Status], c Consumer, status Status, processName string) error {
	for {
		if ctx.Err() != nil {
			return errors.Wrap(ErrWorkflowShutdown, "")
		}

		e, ack, err := c.Recv(ctx)
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

		t0 := w.clock.Now()
		lag := t0.Sub(e.CreatedAt)
		metrics.ConsumerLag.WithLabelValues(w.Name, processName).Set(lag.Seconds())

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

		// If lag alert is set then check if the consumer is lagging and push value of 1 to the lag alert
		// gauge if it is lagging. If it is not lagging then push 0.
		if p.LagAlert > 0 {
			alert := 0.0
			if lag > p.LagAlert {
				alert = 1
			}

			metrics.ConsumerLagAlert.WithLabelValues(w.Name, processName).Set(alert)
		}

		t2 := w.clock.Now()
		err = consume(ctx, record, p.Consumer, ack, p.DestinationStatus, w.endPoints, w.eventStreamerFn, w.recordStore, w.Name, processName)
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t2).Seconds())
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

func consume[Type any, Status StatusType](
	ctx context.Context,
	wr *WireRecord,
	cf ConsumerFunc[Type, Status],
	ack Ack,
	destinationStatus Status,
	endPoints map[Status]bool,
	es EventStreamer,
	rs RecordStore,
	workflowName string,
	processName string,
) error {
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
			"current_status":     Status(wr.Status).String(),
			"current_status_int": wr.Status,
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
	} else {
		metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName).Inc()
	}

	return ack()
}
