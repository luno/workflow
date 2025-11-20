# Configuration

This guide covers all configuration options available when building and running workflows, including step-level options, workflow-level build options, and adapter configurations.

## Overview

Configuration in the workflow system operates at multiple levels:

1. **Step-level options**: Applied to individual steps, callbacks, timeouts, and connectors
2. **Workflow-level options**: Applied to the entire workflow during build
3. **Default options**: Shared defaults that can be overridden per step
4. **Adapter options**: Configuration for infrastructure adapters

## Step-Level Options

Step-level options are applied using the `WithOptions()` method and affect individual workflow components.

### Basic Step Options

```go
builder.AddStep(StatusProcessing,
    processOrderStep,
    StatusCompleted,
).WithOptions(
    workflow.ParallelCount(3),           // Run 3 parallel consumers
    workflow.PollingFrequency(5 * time.Second),  // Poll every 5 seconds
    workflow.ErrBackOff(30 * time.Second),       // 30s backoff on errors
)
```

### Parallel Processing

Control the number of parallel consumers for a step:

```go
// Single consumer (default)
builder.AddStep(StatusValidation, validateStep, StatusValidated)

// Multiple parallel consumers
builder.AddStep(StatusProcessing,
    processStep,
    StatusProcessed,
).WithOptions(
    workflow.ParallelCount(5), // Creates 5 consumers: consumer-1-of-5, consumer-2-of-5, etc.
)
```

### Polling Configuration

Configure how frequently consumers check for new events:

```go
builder.AddStep(StatusWaiting,
    waitStep,
    StatusReady,
).WithOptions(
    workflow.PollingFrequency(100 * time.Millisecond), // High frequency for low latency
)

builder.AddStep(StatusBatchProcessing,
    batchStep,
    StatusCompleted,
).WithOptions(
    workflow.PollingFrequency(30 * time.Second), // Lower frequency for batch operations
)
```

### Error Handling

Configure error backoff and automatic pausing:

```go
builder.AddStep(StatusUploading,
    uploadStep,
    StatusUploaded,
).WithOptions(
    workflow.ErrBackOff(5 * time.Minute),     // Wait 5 minutes after errors
    workflow.PauseAfterErrCount(3),           // Pause after 3 consecutive errors
)
```

### Lag Configuration

Control event processing timing and alerting:

```go
builder.AddStep(StatusNotification,
    notifyStep,
    StatusNotified,
).WithOptions(
    workflow.ConsumeLag(1 * time.Minute),     // Only process events older than 1 minute
    workflow.LagAlert(5 * time.Minute),       // Alert if lag exceeds 5 minutes
)
```

## Workflow Build Options

Workflow-level options are applied during the `Build()` call and affect the entire workflow.

### Basic Build Configuration

```go
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithClock(customClock),
    workflow.WithLogger(customLogger),
    workflow.WithDebugMode(),
    workflow.WithTimeoutStore(timeoutStore),
)
```

### Clock Configuration

Use custom clocks for testing or special timing requirements:

```go
import "k8s.io/utils/clock/testing"

// For testing with controllable time
fakeClock := testing.NewFakeClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithClock(fakeClock),
)

// For production with real time (default)
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithClock(clock.RealClock{}),
)
```

### Logging Configuration

Configure custom logging:

```go
type CustomLogger struct{}

func (c CustomLogger) Debug(ctx context.Context, msg string, meta map[string]string) {
    // Custom debug logging
}

func (c CustomLogger) Error(ctx context.Context, err error) {
    // Custom error logging
}

workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithLogger(CustomLogger{}),
    workflow.WithDebugMode(), // Enable debug logging
)
```

### Timeout Store Configuration

Required when using timeouts:

```go
// Memory store for testing
timeoutStore := memtimeoutstore.New()

// SQL store for production
timeoutStore := sqltimeout.New(db, "timeouts")

workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithTimeoutStore(timeoutStore),
)
```

### Default Options

Set default options that apply to all steps unless overridden:

```go
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithDefaultOptions(
        workflow.PollingFrequency(1 * time.Second),
        workflow.ErrBackOff(10 * time.Second),
        workflow.PauseAfterErrCount(5),
        workflow.LagAlert(2 * time.Minute),
    ),
)
```

### Outbox Configuration

Configure the transactional outbox pattern:

```go
// Enable outbox with custom settings
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithOutboxOptions(
        workflow.OutboxPollingFrequency(500 * time.Millisecond),
        workflow.OutboxErrBackOff(5 * time.Second),
        workflow.OutboxLagAlert(30 * time.Second),
        workflow.OutboxLookupLimit(100), // Process up to 100 events per batch
    ),
)

// Disable outbox entirely
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithoutOutbox(),
)
```

### Debug Mode

Enable verbose logging for development and troubleshooting:

```go
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithDebugMode(), // Enables detailed logging
)
```

## Option Inheritance and Overrides

Options follow a hierarchy where more specific settings override general ones:

```go
// 1. Global defaults (lowest priority)
workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithDefaultOptions(
        workflow.PollingFrequency(30 * time.Second),  // Applied to all steps
        workflow.ErrBackOff(5 * time.Minute),
    ),
)

// 2. Step-specific overrides (highest priority)
builder.AddStep(StatusUrgent,
    urgentStep,
    StatusCompleted,
).WithOptions(
    workflow.PollingFrequency(1 * time.Second),  // Overrides default for this step only
)
```

## Adapter Configuration

### Event Streamers

#### Memory Streamer (Testing)

```go
eventStreamer := memstreamer.New(
    memstreamer.WithClock(customClock),
)
```

#### Kafka Streamer

```go
eventStreamer := kafkastreamer.New(
    brokers,
    kafkastreamer.WithProducerConfig(producerConfig),
    kafkastreamer.WithConsumerConfig(consumerConfig),
)
```

#### Reflex Streamer (Database)

```go
eventStreamer := reflexstreamer.New(
    db,
    eventsTable,
    reflexstreamer.WithCursorsTable(cursorsTable),
)
```

### Record Stores

#### Memory Record Store (Testing)

```go
recordStore := memrecordstore.New(
    memrecordstore.WithClock(customClock),
    memrecordstore.WithOutbox(ctx, eventStreamer, logger),
)
```

#### SQL Record Store

```go
recordStore := sqlrecordstore.New(
    db,
    "workflow_records",
    sqlrecordstore.WithOutbox(ctx, eventStreamer, logger),
)
```

### Timeout Stores

#### Memory Timeout Store (Testing)

```go
timeoutStore := memtimeoutstore.New(
    memtimeoutstore.WithClock(customClock),
)
```

#### SQL Timeout Store

```go
timeoutStore := sqltimeout.New(
    db,
    "workflow_timeouts",
)
```

### Role Schedulers

#### Memory Role Scheduler (Testing)

```go
roleScheduler := memrolescheduler.New()
```

#### Database Role Scheduler

```go
roleScheduler := dbrolescheduler.New(
    db,
    "workflow_roles",
)
```

## Environment-Specific Configuration

### Development Configuration

```go
func buildDevelopmentWorkflow() *workflow.Workflow[Order, Status] {
    eventStreamer := memstreamer.New()
    recordStore := memrecordstore.New()
    roleScheduler := memrolescheduler.New()

    return builder.Build(
        eventStreamer,
        recordStore,
        roleScheduler,
        workflow.WithDebugMode(),                    // Verbose logging
        workflow.WithDefaultOptions(
            workflow.PollingFrequency(100 * time.Millisecond),  // Fast polling
            workflow.ErrBackOff(1 * time.Second),               // Quick error recovery
        ),
    )
}
```

### Testing Configuration

```go
func buildTestWorkflow(t *testing.T) *workflow.Workflow[Order, Status] {
    fakeClock := clock_testing.NewFakeClock(time.Now())
    eventStreamer := memstreamer.New(memstreamer.WithClock(fakeClock))
    recordStore := memrecordstore.New()
    roleScheduler := memrolescheduler.New()

    return builder.Build(
        eventStreamer,
        recordStore,
        roleScheduler,
        workflow.WithClock(fakeClock),               // Controllable time
        workflow.WithDefaultOptions(
            workflow.PollingFrequency(time.Nanosecond),  // Immediate processing
        ),
        workflow.WithoutOutbox(),                    // Simplified for tests
    )
}
```

### Production Configuration

```go
func buildProductionWorkflow(db *sql.DB, kafkaBrokers []string) *workflow.Workflow[Order, Status] {
    eventStreamer := kafkastreamer.New(kafkaBrokers)
    recordStore := sqlrecordstore.New(db, "workflow_records")
    roleScheduler := dbrolescheduler.New(db, "workflow_roles")
    timeoutStore := sqltimeout.New(db, "workflow_timeouts")

    return builder.Build(
        eventStreamer,
        recordStore,
        roleScheduler,
        workflow.WithTimeoutStore(timeoutStore),
        workflow.WithLogger(productionLogger),
        workflow.WithDefaultOptions(
            workflow.PollingFrequency(5 * time.Second),      // Reasonable polling
            workflow.ErrBackOff(30 * time.Second),           // Conservative backoff
            workflow.PauseAfterErrCount(3),                  // Prevent infinite retries
            workflow.LagAlert(5 * time.Minute),              // Monitor lag
        ),
        workflow.WithOutboxOptions(
            workflow.OutboxPollingFrequency(1 * time.Second),  // Fast outbox processing
            workflow.OutboxLookupLimit(500),                   // Larger batches
        ),
    )
}
```

## Configuration Patterns

### High-Throughput Configuration

For workflows that process many events:

```go
builder.AddStep(StatusProcessing,
    processStep,
    StatusCompleted,
).WithOptions(
    workflow.ParallelCount(10),                     // High parallelism
    workflow.PollingFrequency(100 * time.Millisecond),  // Fast polling
    workflow.ErrBackOff(5 * time.Second),           // Quick error recovery
)
```

### Batch Processing Configuration

For workflows that process events in batches:

```go
builder.AddStep(StatusBatching,
    batchStep,
    StatusProcessed,
).WithOptions(
    workflow.ParallelCount(1),                      // Single consumer for batching
    workflow.PollingFrequency(30 * time.Second),    // Slower polling for batching
    workflow.ConsumeLag(5 * time.Minute),           // Wait for batch to accumulate
)
```

### External Integration Configuration

For steps that call external services:

```go
builder.AddStep(StatusAPICall,
    apiCallStep,
    StatusAPIResponse,
).WithOptions(
    workflow.ParallelCount(3),                      // Limited parallelism to avoid overwhelming service
    workflow.ErrBackOff(60 * time.Second),          // Longer backoff for external service issues
    workflow.PauseAfterErrCount(5),                 // Pause on repeated failures
)
```

### Critical Path Configuration

For time-sensitive workflow steps:

```go
builder.AddStep(StatusCritical,
    criticalStep,
    StatusComplete,
).WithOptions(
    workflow.PollingFrequency(100 * time.Millisecond),  // High frequency polling
    workflow.LagAlert(30 * time.Second),                // Quick lag alerting
    workflow.ErrBackOff(1 * time.Second),               // Minimal backoff
)
```

## Configuration Validation

### Validation at Build Time

The workflow system validates configuration at build time:

```go
// This will panic at build time
builder.AddTimeout(StatusWaiting, timer, handler, StatusNext).WithOptions(
    workflow.ParallelCount(2),  // Timeouts don't support parallel processing
)

// This will panic at build time
builder.AddTimeout(StatusWaiting, timer, handler, StatusNext).WithOptions(
    workflow.ConsumeLag(time.Hour),  // Timeouts don't support consume lag
)
```

### Configuration Constraints

- **Timeouts**: Cannot use `ParallelCount()` or `ConsumeLag()`
- **Connectors**: Support all options
- **Steps/Callbacks**: Support all options
- **Parallel Count**: Must be >= 1
- **Polling Frequency**: Must be > 0
- **Error Backoff**: Must be >= 0

## Monitoring Configuration

Configure monitoring and observability:

```go
// Custom metrics configuration would typically be done
// through your metrics collection system (Prometheus, etc.)

workflow := builder.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithDefaultOptions(
        workflow.LagAlert(2 * time.Minute),  // Generates lag alerts
    ),
)

// Monitor metrics:
// - workflow_process_duration_seconds
// - workflow_consumer_lag_seconds
// - workflow_process_skipped_events_total
// - workflow_process_errors_total
```

## Best Practices

### Option Naming

Use descriptive option configurations:

```go
// Good - clear intent
builder.AddStep(StatusEmailSending,
    sendEmailStep,
    StatusEmailSent,
).WithOptions(
    workflow.ParallelCount(5),              // Max 5 concurrent emails
    workflow.ErrBackOff(2 * time.Minute),   // Email service rate limiting
    workflow.PauseAfterErrCount(3),         // Stop on repeated email failures
)

// Avoid - unclear purpose
builder.AddStep(StatusProcessing,
    processStep,
    StatusDone,
).WithOptions(
    workflow.ParallelCount(10),
    workflow.ErrBackOff(30 * time.Second),
)
```

### Environment Separation

Use different configurations per environment:

```go
type Config struct {
    PollingFreq time.Duration
    ErrorBackoff time.Duration
    ParallelCount int
}

func getConfig() Config {
    switch os.Getenv("ENV") {
    case "development":
        return Config{
            PollingFreq: 100 * time.Millisecond,
            ErrorBackoff: 1 * time.Second,
            ParallelCount: 1,
        }
    case "production":
        return Config{
            PollingFreq: 5 * time.Second,
            ErrorBackoff: 30 * time.Second,
            ParallelCount: 5,
        }
    default:
        return Config{
            PollingFreq: 1 * time.Second,
            ErrorBackoff: 10 * time.Second,
            ParallelCount: 2,
        }
    }
}
```

### Resource Management

Consider resource implications of configuration:

```go
// Memory usage scales with parallel count
builder.AddStep(StatusProcessing,
    memoryIntensiveStep,
    StatusCompleted,
).WithOptions(
    workflow.ParallelCount(2),  // Limited due to memory usage
)

// CPU intensive operations
builder.AddStep(StatusComputing,
    cpuIntensiveStep,
    StatusCompleted,
).WithOptions(
    workflow.ParallelCount(runtime.NumCPU()),  // Match CPU cores
)

// I/O bound operations
builder.AddStep(StatusNetworking,
    networkStep,
    StatusCompleted,
).WithOptions(
    workflow.ParallelCount(20),  // Higher parallelism for I/O
)
```