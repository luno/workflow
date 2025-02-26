package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"
	"time"
)

type ConnectorConstructor interface {
	Make(ctx context.Context, consumerName string) (ConnectorConsumer, error)
}

type ConnectorConsumer interface {
	Recv(ctx context.Context) (*ConnectorEvent, Ack, error)
	Close() error
}

type ConnectorFunc[Type any, Status StatusType] func(ctx context.Context, api API[Type, Status], e *ConnectorEvent) error

type connectorConfig[Type any, Status StatusType] struct {
	name        string
	constructor ConnectorConstructor
	connectorFn ConnectorFunc[Type, Status]

	errBackOff    time.Duration
	parallelCount int
	lag           time.Duration
	lagAlert      time.Duration
}

func connectorConsumer[Type any, Status StatusType](
	w *Workflow[Type, Status],
	config *connectorConfig[Type, Status],
	shard, totalShards int,
) {
	role := makeRole(
		config.name,
		"connector",
		"to",
		w.Name(),
		"consumer",
		strconv.FormatInt(int64(shard), 10),
		"of",
		strconv.FormatInt(int64(totalShards), 10),
	)

	errBackOff := w.defaultOpts.errBackOff
	if config.errBackOff > 0 {
		errBackOff = config.errBackOff
	}

	lagAlert := w.defaultOpts.lagAlert
	if config.lagAlert > 0 {
		lagAlert = config.lagAlert
	}

	lag := w.defaultOpts.lag
	if config.lag > 0 {
		lag = config.lag
	}

	// processName can have the same name as the role. It is the same here due to the fact that there are no enums
	// that can be converted to a meaningful string
	processName := role
	w.run(role, processName, func(ctx context.Context) error {
		consumer, err := config.constructor.Make(ctx, role)
		if err != nil {
			return err
		}
		defer consumer.Close()

		return consume(
			ctx,
			w.Name(),
			processName,
			newConnectorStreamer(consumer),
			func(ctx context.Context, e *Event) error {
				ce, err := streamerEventToConnectorEvent(e)
				if err != nil {
					return err
				}

				return config.connectorFn(ctx, w, ce)
			},
			w.clock,
			lag,
			lagAlert,
			shardFilter(shard, totalShards),
		)
	}, errBackOff)
}

type connectorStreamer struct {
	hasher   hash.Hash64
	consumer ConnectorConsumer
}

func newConnectorStreamer(cc ConnectorConsumer) *connectorStreamer {
	return &connectorStreamer{
		hasher:   fnv.New64(),
		consumer: cc,
	}
}

func (c connectorStreamer) Recv(ctx context.Context) (*Event, Ack, error) {
	e, ack, err := c.consumer.Recv(ctx)
	if err != nil {
		return nil, nil, err
	}

	streamerEvent, err := connectorEventToEvent(c.hasher, e)
	if err != nil {
		return nil, nil, err
	}

	return streamerEvent, ack, nil
}

func (c connectorStreamer) Close() error {
	return c.Close()
}

var _ EventReceiver = (*connectorStreamer)(nil)

func streamerEventToConnectorEvent(e *Event) (*ConnectorEvent, error) {
	data, ok := e.Headers[HeaderConnectorData]
	if !ok {
		return nil, fmt.Errorf("no connector data found in event")
	}

	var ce ConnectorEvent
	err := json.Unmarshal([]byte(data), &ce)
	if err != nil {
		return nil, err
	}

	return &ce, nil
}

func connectorEventToEvent(hasher hash.Hash64, e *ConnectorEvent) (*Event, error) {
	hasher.Reset()
	_, err := hasher.Write([]byte(e.ID))
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	// Not all fields are populated connectorEventToEvent is tightly coupled the consumer above and how it works.
	return &Event{
		ID:        int64(hasher.Sum64()), // Event ID is only used for consistent sharding filter
		ForeignID: e.ForeignID,
		Headers: map[Header]string{
			HeaderConnectorData: string(b),
		},
		CreatedAt: e.CreatedAt,
	}, nil
}
