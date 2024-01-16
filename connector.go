package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"

	"github.com/luno/workflow/internal/metrics"
)

// ConnectorFilter should return an empty string as the foreignID if the event should be filtered out / skipped, and
// it should be non-empty if event should be processed. The value of foreignID should match the foreignID of your
// workflow.
type ConnectorFilter func(ctx context.Context, e *Event) (foreignID string, err error)

type ConnectorConsumerFunc[Type any, Status StatusType] func(ctx context.Context, r *Record[Type, Status], e *Event) (bool, error)

type ConnectorFunc[Type any, Status StatusType] func(ctx context.Context, w *Workflow[Type, Status], e *Event) error

type WorkflowConnectionDetails struct {
	WorkflowName string
	Status       int
	Stream       EventStreamer
}

type connectorConfig[Type any, Status StatusType] struct {
	name        string
	consumerFn  Consumer
	connectorFn ConnectorFunc[Type, Status]

	errBackOff    time.Duration
	parallelCount int
}

type workflowConnectorConfig[Type any, Status StatusType] struct {
	workflowName     string
	status           int
	stream           EventStreamer
	filter           ConnectorFilter
	consumer         ConnectorConsumerFunc[Type, Status]
	from             Status
	to               Status
	pollingFrequency time.Duration
	errBackOff       time.Duration
	parallelCount    int
}

type connectorOptions struct {
	parallelCount    int
	pollingFrequency time.Duration
	errBackOff       time.Duration
}

type ConnectorOption func(co *connectorOptions)

func WithConnectorParallelCount(instances int) ConnectorOption {
	return func(co *connectorOptions) {
		co.parallelCount = instances
	}
}

func WithConnectorErrBackOff(d time.Duration) ConnectorOption {
	return func(co *connectorOptions) {
		co.errBackOff = d
	}
}

func connectorConsumer[Type any, Status StatusType](w *Workflow[Type, Status], cc *connectorConfig[Type, Status], shard, totalShards int) {
	role := makeRole(
		cc.name,
		"connector",
		"to",
		w.Name,
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	// processName can have the same name as the role. It is the same here due to the fact that there are no enums
	// that can be converted to a meaningful string
	processName := role

	errBackOff := w.defaultErrBackOff
	if cc.errBackOff.Nanoseconds() != 0 {
		errBackOff = cc.errBackOff
	}

	defer cc.consumerFn.Close()
	w.run(role, processName, func(ctx context.Context) error {
		e, ack, err := cc.consumerFn.Recv(ctx)
		if err != nil {
			return err
		}

		err = cc.connectorFn(ctx, w, e)
		if err != nil {
			return errors.Wrap(err, "failed to consume - connector consumer", j.MKV{
				"workflow_name":    w.Name,
				"event_foreign_id": e.ForeignID,
				"event_type":       e.Type,
				"role":             cc.name,
			})
		}

		return ack()
	}, errBackOff)
}

func workflowConnectorConsumer[Type any, Status StatusType](w *Workflow[Type, Status], cc *workflowConnectorConfig[Type, Status], shard, totalShards int) {
	role := makeRole(
		cc.workflowName,
		fmt.Sprintf("%v", cc.status),
		"connection",
		w.Name,
		fmt.Sprintf("%v", int(cc.from)),
		"to",
		fmt.Sprintf("%v", int(cc.to)),
		"connector",
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	processName := makeRole(
		cc.workflowName,
		fmt.Sprintf("%v", cc.status),
		"connection",
		w.Name,
		cc.from.String(),
		"to",
		cc.to.String(),
		"connector",
		"consumer",
		fmt.Sprintf("%v", shard),
		"of",
		fmt.Sprintf("%v", totalShards),
	)

	pollFrequency := w.defaultPollingFrequency
	if cc.pollingFrequency.Nanoseconds() != 0 {
		pollFrequency = cc.pollingFrequency
	}

	errBackOff := w.defaultErrBackOff
	if cc.errBackOff.Nanoseconds() != 0 {
		errBackOff = cc.errBackOff
	}

	topic := Topic(cc.workflowName, cc.status)
	stream := cc.stream.NewConsumer(
		topic,
		role,
		WithConsumerPollFrequency(pollFrequency),
		WithEventFilter(
			shardFilter(shard, totalShards),
		),
	)
	defer stream.Close()

	w.run(role, processName, func(ctx context.Context) error {
		return consumeExternalWorkflow[Type, Status](ctx, stream, w, cc.workflowName, cc.status, cc.filter, cc.consumer, cc.to, processName)
	}, errBackOff)
}

func consumeExternalWorkflow[Type any, Status StatusType](ctx context.Context, stream Consumer, w *Workflow[Type, Status], externalWorkflowName string, status int, filter ConnectorFilter, consumerFunc ConnectorConsumerFunc[Type, Status], to Status, processName string) error {
	for {
		if ctx.Err() != nil {
			return errors.Wrap(ErrWorkflowShutdown, "")
		}

		e, ack, err := stream.Recv(ctx)
		if err != nil {
			return err
		}

		if e.Headers[HeaderWorkflowName] != externalWorkflowName {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		if e.Type != status {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		foreignID, err := filter(ctx, e)
		if err != nil {
			return err
		}

		if foreignID == "" {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		latest, err := w.recordStore.Latest(ctx, w.Name, foreignID)
		if err != nil {
			return err
		}

		var t Type
		err = Unmarshal(latest.Object, &t)
		if err != nil {
			return err
		}

		record := Record[Type, Status]{
			WireRecord: *latest,
			Status:     Status(latest.Status),
			Object:     &t,
		}

		t0 := w.clock.Now()
		ok, err := consumerFunc(ctx, &record, e)
		if err != nil {
			return errors.Wrap(err, "failed to consume - connector consumer", j.MKV{
				"workflow_name":      latest.WorkflowName,
				"foreign_id":         latest.ForeignID,
				"current_status":     latest.Status,
				"destination_status": to,
			})
		}
		metrics.ProcessLatency.WithLabelValues(w.Name, processName).Observe(w.clock.Since(t0).Seconds())

		if ok {
			b, err := Marshal(&record.Object)
			if err != nil {
				return err
			}

			isEnd := w.endPoints[to]
			wr := &WireRecord{
				ID:           record.ID,
				RunID:        record.RunID,
				WorkflowName: record.WorkflowName,
				ForeignID:    record.ForeignID,
				Status:       int(to),
				IsStart:      false,
				IsEnd:        isEnd,
				Object:       b,
				CreatedAt:    record.CreatedAt,
			}

			err = update(ctx, w.eventStreamerFn, w.recordStore, wr)
			if err != nil {
				return err
			}
		}

		return ack()
	}
}
