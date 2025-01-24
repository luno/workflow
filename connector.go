package workflow

import (
	"context"
	"strconv"
	"time"

	"github.com/luno/workflow/internal/metrics"
)

type ConnectorConstructor interface {
	Make(ctx context.Context, consumerName string) (ConnectorConsumer, error)
}

type ConnectorConsumer interface {
	Recv(ctx context.Context) (*ConnectorEvent, Ack, error)
	Close() error
}

type ConnectorFunc[Type any, Status StatusType] func(ctx context.Context, w API[Type, Status], e *ConnectorEvent) error

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

		return connectForever(ctx, w, config.connectorFn, lag, lagAlert, consumer, processName, shard, totalShards)
	}, errBackOff)
}

func connectForever[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	connectorFn ConnectorFunc[Type, Status],
	lag time.Duration,
	lagAlert time.Duration,
	consumer ConnectorConsumer,
	processName string,
	shard, totalShards int,
) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e, ack, err := consumer.Recv(ctx)
		if err != nil {
			return err
		}

		// Wait until the event's timestamp matches or is older than the specified lag.
		delay := lag - w.clock.Since(e.CreatedAt)
		if lag > 0 && delay > 0 {
			t := w.clock.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C():
				// Resume to consume the event now that it matches or is older than specified lag.
			}
		}

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(w.Name(), processName, e.CreatedAt, lagAlert, w.clock)

		shouldFilter := FilterConnectorEventUsing(e,
			shardConnectorEventFilter(shard, totalShards),
		)
		if shouldFilter {
			err = ack()
			if err != nil {
				return err
			}

			continue
		}

		t2 := w.clock.Now()
		err = connectorFn(ctx, w, e)
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(w.Name(), processName).Observe(w.clock.Since(t2).Seconds())
		return ack()
	}
}
