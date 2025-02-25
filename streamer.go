package workflow

import (
	"context"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
)

func Consume(
	ctx context.Context,
	workflowName string,
	processName string,
	stream Consumer,
	consumeFn func(ctx context.Context, e *Event) error,
	clock clock.Clock,
	lag time.Duration,
	lagAlert time.Duration,
	filters ...EventFilter,
) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		e, ack, err := stream.Recv(ctx)
		if err != nil {
			return err
		}

		// Wait until the event's timestamp matches or is older than the specified lag.
		delay := lag - clock.Since(e.CreatedAt)
		if lag > 0 && delay > 0 {
			t := clock.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C():
				// Resume to consume the event now that it matches or is older than specified lag.
			}
		}

		// Push metrics and alerting around the age of the event being processed.
		pushLagMetricAndAlerting(workflowName, processName, e.CreatedAt, lagAlert, clock)

		shouldFilter := FilterUsing(e, filters...)
		if shouldFilter {
			err = ack()
			if err != nil {
				return err
			}

			metrics.ProcessSkippedEvents.WithLabelValues(workflowName, processName, "filtered out").Inc()
			continue
		}

		t0 := clock.Now()
		err = consumeFn(ctx, e)
		if err != nil {
			return err
		}

		err = ack()
		if err != nil {
			return err
		}

		metrics.ProcessLatency.WithLabelValues(workflowName, processName).Observe(clock.Since(t0).Seconds())
	}
}

// pushLagMetricAndAlerting will push metrics around the age of the event being processed. If the age of the event is
// greater than the threshold then the processName for the workflow specified (workflowName) will be set to 1 which
// signals that this process for this workflow is in an alerting state.
//
// See internal/metrics/metrics.go for the prometheus metrics configured.
func pushLagMetricAndAlerting(
	workflowName string,
	processName string,
	timestamp time.Time,
	lagThreshold time.Duration,
	clock clock.Clock,
) {
	t0 := clock.Now()
	lag := t0.Sub(timestamp)
	metrics.ConsumerLag.WithLabelValues(workflowName, processName).Set(lag.Seconds())

	// If lag alert is set then check if the consumer is lagging and push value of 1 to the lag alert
	// gauge if it is lagging. If it is not lagging then push 0.
	if lagThreshold > 0 {
		alert := 0.0
		if lag > lagThreshold {
			alert = 1
		}

		metrics.ConsumerLagAlert.WithLabelValues(workflowName, processName).Set(alert)
	}
}
