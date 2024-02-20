package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
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
