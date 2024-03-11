package workflow

import (
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
)

// pushLagMetricAndAlerting will push metrics around the age of the event being processed. If the age of the event is
// greater than the threshold then the processName for the workflow specified (workflowName) will be set to 1 which
// signals that this process for this workflow is in an alerting state.
//
// See internal/metrics/metrics.go for the prometheus metrics configured.
func pushLagMetricAndAlerting(workflowName string, processName string, timestamp time.Time, lagThreshold time.Duration, clock clock.Clock) {
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
