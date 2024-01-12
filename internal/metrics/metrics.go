package metrics

import "github.com/prometheus/client_golang/prometheus"

const (
	workflowName = "workflow_name"
	processName  = "process_name"
)

var (
	// ConsumerLag is a metric for how far behind the consumer is
	// based on the last consumed event
	ConsumerLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "workflow_process_lag_seconds",
		Help: "Lag between now and the current event timestamp in seconds",
	}, []string{workflowName, processName})

	// ConsumerLagAlert is whether the consumer is too far behind or not
	ConsumerLagAlert = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "workflow_process_lag_alert",
		Help: "Whether or not the consumer lag crosses its alert threshold",
	}, []string{workflowName, processName})

	// ProcessStates reflects the states of all the processes for the instance that form part of the workflow
	ProcessStates = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "workflow_process_states",
		Help: "The current states of all the processes",
	}, []string{workflowName, processName})

	// ProcessLatency is how long the process is taking to process an event
	ProcessLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "workflow_process_latency_seconds",
		Help:    "Event loop latency in seconds",
		Buckets: []float64{0.01, 0.1, 1, 5, 10, 60, 300},
	}, []string{workflowName, processName})

	// ProcessErrors is the number of errors from processing events
	ProcessErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "workflow_process_error_count",
		Help: "Number of errors processing events",
	}, []string{workflowName, processName})

	// ProcessSkippedEvents is the number of events skipped by the process
	ProcessSkippedEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "workflow_process_skipped_events_count",
		Help: "Number of events skipped by consumer",
	}, []string{workflowName, processName})
)

func init() {
	prometheus.MustRegister(
		ConsumerLag,
		ConsumerLagAlert,
		ProcessStates,
		ProcessLatency,
		ProcessErrors,
		ProcessSkippedEvents,
	)
}

func Reset() {
	ConsumerLag.Reset()
	ConsumerLagAlert.Reset()
	ProcessStates.Reset()
	ProcessLatency.Reset()
	ProcessErrors.Reset()
	ProcessSkippedEvents.Reset()
}
