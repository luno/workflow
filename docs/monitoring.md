# Monitoring

This guide covers monitoring and observability features built into the workflow system, including Prometheus metrics, logging, alerting, and troubleshooting workflows in production.

## Overview

The workflow system provides comprehensive observability through:

1. **Prometheus Metrics**: Built-in metrics for performance and health monitoring
2. **Structured Logging**: Configurable logging with debug modes
3. **Process State Tracking**: Real-time visibility into workflow processes
4. **Lag Monitoring**: Consumer lag detection and alerting
5. **Error Tracking**: Detailed error metrics and patterns

## Built-in Metrics

The workflow system automatically exports Prometheus metrics that provide insights into performance, errors, and system health.

### Process Metrics

#### `workflow_process_latency_seconds`

Histogram tracking how long each process takes to handle an event.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process (e.g., "processing-consumer")

**Buckets:** `[0.01, 0.1, 1, 5, 10, 60, 300]` seconds

```promql
# Average processing time per workflow
rate(workflow_process_latency_seconds_sum[5m]) / rate(workflow_process_latency_seconds_count[5m])

# 95th percentile processing time
histogram_quantile(0.95, rate(workflow_process_latency_seconds_bucket[5m]))
```

#### `workflow_process_error_count`

Counter tracking the number of errors encountered during event processing.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process

```promql
# Error rate per workflow
rate(workflow_process_error_count[5m])

# Workflows with highest error rates
topk(5, rate(workflow_process_error_count[5m]))
```

#### `workflow_process_skipped_events_count`

Counter tracking events that were skipped by consumers with reasons.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process
- `reason`: Why the event was skipped

**Common skip reasons:**
- `"record not found"`: Record was deleted before processing
- `"record status not in expected state"`: Record status changed before processing
- `"event record version lower than latest record version"`: Stale event
- `"record stopped"`: Record is in a stopped state
- `"next value specified skip"`: Step function returned skip
- `"filtered out"`: Event filtered by custom logic

```promql
# Events skipped per reason
rate(workflow_process_skipped_events_count[5m]) by (reason)

# Skip rate percentage
(rate(workflow_process_skipped_events_count[5m]) / (rate(workflow_process_skipped_events_count[5m]) + rate(workflow_process_latency_seconds_count[5m]))) * 100
```

#### `workflow_process_states`

Gauge showing the current state of workflow processes.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process

**Values:**
- `0`: Stopped
- `1`: Running
- `2`: Error

```promql
# Processes in error state
workflow_process_states == 2

# Healthy running processes
workflow_process_states == 1
```

### Lag Metrics

#### `workflow_process_lag_seconds`

Gauge showing the lag between current time and the timestamp of the last consumed event.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process

```promql
# Consumers with highest lag
topk(10, workflow_process_lag_seconds)

# Lag trending over time
increase(workflow_process_lag_seconds[1h])
```

#### `workflow_process_lag_alert`

Boolean gauge indicating whether consumer lag exceeds the configured alert threshold.

**Labels:**
- `workflow_name`: Name of the workflow
- `process_name`: Name of the process

**Values:**
- `0`: Lag is below alert threshold
- `1`: Lag exceeds alert threshold

```promql
# Consumers in lag alert state
workflow_process_lag_alert == 1

# Count of lagging consumers
count(workflow_process_lag_alert == 1)
```

### Workflow State Metrics

#### `workflow_run_state_changes`

Counter tracking transitions between run states for workflow instances.

**Labels:**
- `workflow_name`: Name of the workflow
- `previous_run_state`: Previous RunState
- `current_run_state`: Current RunState

**Run States:**
- `"Unknown"`: Initial state (should be minimal)
- `"Initiated"`: Workflow started
- `"Running"`: Workflow processing
- `"Paused"`: Workflow paused due to errors
- `"Cancelled"`: Workflow cancelled
- `"Completed"`: Workflow completed successfully
- `"DataDeleted"`: Workflow data deleted

```promql
# Rate of workflow completions
rate(workflow_run_state_changes{current_run_state="Completed"}[5m])

# Rate of workflow failures (paused/cancelled)
rate(workflow_run_state_changes{current_run_state=~"Paused|Cancelled"}[5m])

# Success rate percentage
(rate(workflow_run_state_changes{current_run_state="Completed"}[5m]) / rate(workflow_run_state_changes{current_run_state=~"Completed|Paused|Cancelled"}[5m])) * 100
```

## Monitoring Dashboards

### Workflow Health Dashboard

Key metrics for overall workflow system health:

```yaml
panels:
  - title: "Workflow Throughput"
    query: rate(workflow_process_latency_seconds_count[5m])

  - title: "Error Rate"
    query: rate(workflow_process_error_count[5m])

  - title: "Consumer Lag"
    query: workflow_process_lag_seconds

  - title: "Process States"
    query: workflow_process_states

  - title: "Completion Rate"
    query: rate(workflow_run_state_changes{current_run_state="Completed"}[5m])
```

### Performance Dashboard

Detailed performance metrics:

```yaml
panels:
  - title: "Processing Latency P95"
    query: histogram_quantile(0.95, rate(workflow_process_latency_seconds_bucket[5m]))

  - title: "Processing Latency P50"
    query: histogram_quantile(0.5, rate(workflow_process_latency_seconds_bucket[5m]))

  - title: "Event Skip Rate"
    query: rate(workflow_process_skipped_events_count[5m])

  - title: "Skip Reasons"
    query: rate(workflow_process_skipped_events_count[5m]) by (reason)
```

### Workflow-Specific Dashboard

Monitor individual workflows:

```yaml
variables:
  - name: "workflow"
    query: label_values(workflow_process_latency_seconds, workflow_name)

panels:
  - title: "Throughput for {{workflow}}"
    query: rate(workflow_process_latency_seconds_count{workflow_name="$workflow"}[5m])

  - title: "Errors for {{workflow}}"
    query: rate(workflow_process_error_count{workflow_name="$workflow"}[5m])

  - title: "Lag for {{workflow}}"
    query: workflow_process_lag_seconds{workflow_name="$workflow"}
```

## Alerting

### Critical Alerts

High-priority alerts for immediate attention:

```yaml
# High Error Rate
- alert: WorkflowHighErrorRate
  expr: rate(workflow_process_error_count[5m]) > 0.1
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "High error rate in workflow {{ $labels.workflow_name }}"
    description: "Error rate is {{ $value }} errors/sec for {{ $labels.process_name }}"

# Consumer Lag Alert
- alert: WorkflowConsumerLag
  expr: workflow_process_lag_alert == 1
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Consumer lag alert for {{ $labels.workflow_name }}"
    description: "Consumer {{ $labels.process_name }} lag exceeds threshold"

# Process Down
- alert: WorkflowProcessDown
  expr: workflow_process_states == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Workflow process {{ $labels.process_name }} is down"
    description: "Process has been stopped for {{ $labels.workflow_name }}"
```

### Warning Alerts

Medium-priority alerts for investigation:

```yaml
# Increased Skip Rate
- alert: WorkflowHighSkipRate
  expr: (rate(workflow_process_skipped_events_count[5m]) / rate(workflow_process_latency_seconds_count[5m])) > 0.2
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High skip rate for {{ $labels.workflow_name }}"
    description: "Skip rate is {{ $value | humanizePercentage }} for {{ $labels.process_name }}"

# Slow Processing
- alert: WorkflowSlowProcessing
  expr: histogram_quantile(0.95, rate(workflow_process_latency_seconds_bucket[5m])) > 10
  for: 3m
  labels:
    severity: warning
  annotations:
    summary: "Slow processing in {{ $labels.workflow_name }}"
    description: "95th percentile latency is {{ $value }}s for {{ $labels.process_name }}"

# Low Completion Rate
- alert: WorkflowLowCompletionRate
  expr: (rate(workflow_run_state_changes{current_run_state="Completed"}[5m]) / rate(workflow_run_state_changes{current_run_state=~"Completed|Paused|Cancelled"}[5m])) < 0.9
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Low completion rate for {{ $labels.workflow_name }}"
    description: "Completion rate is {{ $value | humanizePercentage }}"
```

## Logging

### Log Levels

The workflow system provides structured logging with different levels:

```go
// Configure custom logger
type CustomLogger struct {
    logger *slog.Logger
}

func (c CustomLogger) Debug(ctx context.Context, msg string, meta map[string]string) {
    c.logger.DebugContext(ctx, msg, slog.Any("meta", meta))
}

func (c CustomLogger) Error(ctx context.Context, err error) {
    c.logger.ErrorContext(ctx, "Workflow error", slog.Any("error", err))
}

// Enable debug logging
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithLogger(CustomLogger{logger: slog.Default()}),
    workflow.WithDebugMode(), // Required for debug logs
)
```

### Debug Mode

Enable detailed logging for development and troubleshooting:

```go
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithDebugMode(),
)
```

Debug mode logs include:
- Process lifecycle events (start/stop)
- Event processing details
- Skip reasons with context
- Error details with stack traces
- Consumer lag warnings

### Log Correlation

Use correlation IDs for tracing workflow execution:

```go
func stepWithLogging(ctx context.Context, r *workflow.Run[Order, Status]) (Status, error) {
    logger := slog.With(
        "workflow", "order-processing",
        "foreign_id", r.ForeignID,
        "run_id", r.RunID,
        "current_status", r.Status.String(),
    )

    logger.InfoContext(ctx, "Processing order", "order_value", r.Object.Total)

    err := processOrder(ctx, r.Object)
    if err != nil {
        logger.ErrorContext(ctx, "Failed to process order", "error", err)
        return 0, err
    }

    logger.InfoContext(ctx, "Order processed successfully")
    return StatusProcessed, nil
}
```

## Troubleshooting

### Common Issues and Metrics

#### High Error Rate

```promql
# Check error patterns
rate(workflow_process_error_count[5m]) by (workflow_name, process_name)

# Check if specific processes are failing
workflow_process_states == 2
```

**Investigation steps:**
1. Check application logs for error details
2. Verify external service availability
3. Check resource utilization (CPU, memory)
4. Review recent deployments or configuration changes

#### Consumer Lag

```promql
# Identify lagging consumers
workflow_process_lag_seconds > 300

# Check lag trend
increase(workflow_process_lag_seconds[1h])
```

**Investigation steps:**
1. Check if processing is slower than event arrival rate
2. Verify database performance
3. Consider increasing parallel consumers
4. Check for resource constraints

#### High Skip Rate

```promql
# Skip rate by reason
rate(workflow_process_skipped_events_count[5m]) by (reason)

# Skip rate percentage
(rate(workflow_process_skipped_events_count[5m]) / rate(workflow_process_latency_seconds_count[5m])) * 100
```

**Common causes:**
- Race conditions (events processed out of order)
- Data cleanup deleting records before processing
- Logic errors in step functions
- Upstream system issues

#### Low Throughput

```promql
# Processing rate
rate(workflow_process_latency_seconds_count[5m])

# Processing latency
histogram_quantile(0.95, rate(workflow_process_latency_seconds_bucket[5m]))
```

**Investigation steps:**
1. Check processing latency metrics
2. Review CPU and memory usage
3. Analyze database query performance
4. Consider horizontal scaling

### Runbook Examples

#### Consumer Lag Alert

```bash
# 1. Check current lag
curl -s "http://prometheus:9090/api/v1/query?query=workflow_process_lag_seconds" | jq '.data.result'

# 2. Check event processing rate
curl -s "http://prometheus:9090/api/v1/query?query=rate(workflow_process_latency_seconds_count[5m])" | jq '.data.result'

# 3. Scale up consumers if needed
kubectl scale deployment workflow-service --replicas=5

# 4. Monitor lag improvement
watch "curl -s 'http://prometheus:9090/api/v1/query?query=workflow_process_lag_seconds' | jq '.data.result[].value[1]'"
```

#### High Error Rate

```bash
# 1. Identify failing workflow
curl -s "http://prometheus:9090/api/v1/query?query=rate(workflow_process_error_count[5m])" | jq '.data.result'

# 2. Check process states
curl -s "http://prometheus:9090/api/v1/query?query=workflow_process_states" | jq '.data.result'

# 3. Check application logs
kubectl logs -f deployment/workflow-service --tail=100

# 4. Restart if needed
kubectl rollout restart deployment/workflow-service
```

## Custom Metrics

### Application-Specific Metrics

Add your own business metrics alongside workflow metrics:

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    orderValue = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "order_processing_value_dollars",
            Help: "Value of orders being processed",
            Buckets: []float64{10, 50, 100, 500, 1000, 5000},
        },
        []string{"workflow_name"},
    )

    paymentFailures = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "order_payment_failures_total",
            Help: "Number of payment failures",
        },
        []string{"workflow_name", "payment_provider"},
    )
)

func init() {
    prometheus.MustRegister(orderValue, paymentFailures)
}

func processPayment(ctx context.Context, r *workflow.Run[Order, Status]) (Status, error) {
    // Record business metrics
    orderValue.WithLabelValues("order-processing").Observe(float64(r.Object.Total))

    err := chargePayment(ctx, r.Object)
    if err != nil {
        // Track payment failures by provider
        paymentFailures.WithLabelValues("order-processing", r.Object.PaymentProvider).Inc()
        return 0, err
    }

    return StatusPaymentProcessed, nil
}
```

### Metrics Export

Expose metrics endpoint for Prometheus scraping:

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "net/http"
)

func main() {
    // Start metrics server
    go func() {
        http.Handle("/metrics", promhttp.Handler())
        http.ListenAndServe(":8080", nil)
    }()

    // Start workflow
    workflow.Run(context.Background())
}
```

Configure Prometheus to scrape metrics:

```yaml
scrape_configs:
  - job_name: 'workflow-service'
    static_configs:
      - targets: ['workflow-service:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Performance Monitoring

### Key Performance Indicators

Monitor these KPIs for workflow system health:

1. **Throughput**: Events processed per second
2. **Latency**: Time to process each event (P50, P95, P99)
3. **Error Rate**: Percentage of failed events
4. **Success Rate**: Percentage of completed workflows
5. **Lag**: Consumer lag in seconds
6. **Skip Rate**: Percentage of skipped events

### Capacity Planning

Use metrics to plan scaling:

```promql
# Throughput (requests/sec):
rate(workflow_process_latency_seconds_count[5m])

# CPU Usage (seconds/sec):
increase(cpu_usage_seconds[5m])

# Concurrent Workflows:
count(workflow_run_state_changes{current_run_state="Running"})

# Memory Usage (bytes):
memory_usage_bytes

# Processing Lag (seconds):
workflow_process_lag_seconds

# Processing Rate (events/sec):
rate(workflow_process_latency_seconds_count[5m])
```

## Best Practices

### Metric Naming

Follow Prometheus naming conventions:

```go
// Good
workflow_process_duration_seconds_total
order_payment_attempts_total
customer_notification_sent_total

// Avoid
ProcessingTime
paymentAttempts
NotificationsSent
```

### Alert Thresholds

Set appropriate thresholds based on SLAs:

```yaml
# For critical workflows (< 1 minute SLA)
- expr: workflow_process_lag_seconds > 30

# For batch workflows (< 1 hour SLA)
- expr: workflow_process_lag_seconds > 1800

# Error rate based on volume
- expr: rate(workflow_process_error_count[5m]) > 0.1  # 10% error rate
```

### Dashboard Organization

Structure dashboards by audience:

1. **Executive Dashboard**: High-level business metrics
2. **Operations Dashboard**: System health and alerts
3. **Development Dashboard**: Detailed debugging metrics
4. **Workflow-Specific**: Per-workflow deep dive

### Log Retention

Configure appropriate retention for different log levels:

```yaml
# Structured logging configuration
debug_logs:
  retention: 7 days
  volume: high

error_logs:
  retention: 90 days
  volume: medium

audit_logs:
  retention: 365 days
  volume: low
```