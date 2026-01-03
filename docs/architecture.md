# Architecture

This document provides a deep dive into Workflow's architecture, design decisions, and how the components work together to provide a robust, scalable workflow orchestration system.

## System Overview

Workflow is built around several core principles:

- **Event-Driven**: All communication between components happens through events
- **Distributed-First**: Designed to run across multiple instances from day one
- **Type-Safe**: Leverages Go's type system for compile-time correctness
- **Adapter-Based**: Infrastructure-agnostic through pluggable adapters
- **Durable**: All state changes are persisted with transactional guarantees

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Application                       │
└─────────────────┬───────────────────────┬─────────────────────┘
                  │                       │
                  ▼                       ▼
         ┌─────────────────┐    ┌─────────────────┐
         │   Workflow API  │    │   Web UI        │
         │   - Trigger()   │    │   - Monitor     │
         │   - Await()     │    │   - Debug       │
         │   - Callback()  │    │   - Visualize   │
         └─────────┬───────┘    └─────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Workflow Engine                          │
├─────────────────┬───────────────┬───────────────┬───────────────┤
│   Step          │   Callback    │   Timeout     │   Connector   │
│   Consumers     │   Handlers    │   Pollers     │   Consumers   │
│                 │               │               │               │
│   Process       │   Handle      │   Schedule    │   Consume     │
│   workflow      │   external    │   time-based  │   external    │
│   steps         │   events      │   operations  │   streams     │
└─────────┬───────┴───────┬───────┴───────┬───────┴───────┬───────┘
          │               │               │               │
          ▼               ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Role Scheduler                             │
│   Coordinates distributed execution of workflow processes      │
│   - Ensures only one instance of each role runs at a time     │
│   - Handles failover and load distribution                    │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Adapters                                 │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────┤
│EventStreamer│RecordStore  │TimeoutStore │   Logger    │WebUI    │
│             │             │             │             │         │
│ - Kafka     │ - SQL DB    │ - SQL DB    │ - Structured│ - HTTP  │
│ - Reflex    │ - NoSQL     │ - Redis     │ - Debug     │ - React │
│ - Memory    │ - Memory    │ - Memory    │ - Custom    │ - Custom│
└─────────────┴─────────────┴─────────────┴─────────────┴─────────┘
```

## Core Components

### Workflow Engine

The engine is responsible for:
- Managing workflow definitions and runtime state
- Coordinating step execution across distributed instances
- Handling error recovery and retry logic
- Providing the public API for triggering and monitoring workflows

### Event-Driven Communication

All components communicate through events, providing:

**Loose Coupling**: Components don't need to know about each other directly

**Reliability**: Events are persisted and can be replayed on failure

**Observability**: All state changes are observable as events

**Scalability**: Events can be partitioned and processed in parallel

### Role-Based Scheduling

The role scheduler ensures distributed coordination:

```go
type RoleScheduler interface {
    Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error)
}
```

**How it works:**
1. Each workflow process defines a unique role
2. Scheduler ensures only one instance holds each role
3. If an instance fails, scheduler reassigns roles to healthy instances
4. Load can be distributed by creating multiple roles for the same logical process

**Role Naming Convention:**
```
{workflow-name}-{status}-{process-type}-{shard}-of-{total-shards}
```

Examples:
- `order-processing-1-consumer-1-of-1`
- `user-onboarding-2-timeout-consumer-2-of-3`

## Data Flow

### 1. Workflow Trigger

```go
runID, err := workflow.Trigger(ctx, "order-123",
    workflow.WithInitialValue(order),
    workflow.WithStartAt(OrderCreated),
)
```

1. Creates a new Run record in RecordStore
2. Publishes event to EventStreamer
3. Returns immediately (async processing)

### 2. Event Processing

```
EventStreamer → Step Consumer → Business Logic → State Update → New Event
```

1. Step consumer receives event for its status
2. Loads current Run state from RecordStore
3. Executes step function with business logic
4. Updates Run state and publishes new event
5. Process repeats for next step

### 3. Transactional Outbox

To ensure exactly-once processing:

```go
tx := db.Begin()
// 1. Update run state
recordStore.Store(tx, updatedRun)
// 2. Write event to outbox
recordStore.AddOutboxEvent(tx, event)
tx.Commit()

// 3. Async: publish outbox events
outboxProcessor.PublishPending()
```

This pattern ensures that state changes and events are atomically committed.

## Scaling Patterns

### Horizontal Scaling

Workflow supports several scaling patterns:

**Single Instance**: All roles run on one machine
```go
// Simple setup - everything on one instance
wf := b.Build(memstreamer.New(), memrecordstore.New(), memrolescheduler.New())
```

**Multi-Instance**: Roles distributed across machines
```go
// Production setup - roles distributed via RoleScheduler
wf := b.Build(kafkastreamer.New(), sqlstore.New(), rinkrolescheduler.New())
```

**Sharded Processing**: High-throughput steps split across multiple instances
```go
b.AddStep(OrderCreated, processPayment, PaymentProcessed).
    WithOptions(workflow.ParallelCount(5)) // 5 parallel processors
```

### Auto-Scaling Considerations

- **Stateless Processes**: All workflow processes are stateless and can be scaled independently
- **Role Rebalancing**: Role scheduler automatically redistributes work when instances are added/removed
- **Graceful Shutdown**: Instances finish current work before shutting down
- **Circuit Breakers**: Built-in pause mechanism prevents cascade failures

## Adapter Architecture

Adapters provide infrastructure abstraction:

### EventStreamer Interface

```go
type EventStreamer interface {
    NewSender(ctx context.Context, topic string) (EventSender, error)
    NewReceiver(ctx context.Context, topic string, name string, opts ...ReceiverOption) (EventReceiver, error)
}

type EventSender interface {
    Send(ctx context.Context, foreignID string, statusType int, headers map[Header]string) error
    Close() error
}

type EventReceiver interface {
    Recv(ctx context.Context) (*Event, Ack, error)
    Close() error
}
```

**Implementations:**
- **kafkastreamer**: Production-ready Kafka integration
- **reflexstreamer**: Integration with Luno's Reflex event sourcing
- **memstreamer**: In-memory for testing and development

### RecordStore Interface

```go
type RecordStore interface {
    Store(ctx context.Context, record *Record) error
    Lookup(ctx context.Context, runID string) (*Record, error)
    Latest(ctx context.Context, workflowName, foreignID string) (*Record, error)
    List(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error)
    ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]OutboxEvent, error)
    DeleteOutboxEvent(ctx context.Context, id string) error
}
```

**Key Requirements:**
- **Transactional**: Must support transactions for outbox pattern
- **Query Capabilities**: Support filtering and pagination
- **Performance**: Optimized for workflow access patterns

## Error Handling & Resilience

### Multi-Level Error Handling

**Step Level**: Individual step errors
```go
func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    if err := paymentService.Charge(r.Object.Amount); err != nil {
        if isRetryableError(err) {
            return 0, err // Retry with backoff
        }
        return PaymentFailed, nil // Move to failure state
    }
    return PaymentProcessed, nil
}
```

**Process Level**: Consumer and timeout process errors
- Automatic retries with exponential backoff
- Pause after configurable error count
- Role reassignment on persistent failures

**System Level**: Infrastructure failures
- Event replay from last checkpoint
- Role failover to healthy instances
- Circuit breakers to prevent cascade failures

### Error Isolation

- Errors in one workflow don't affect others
- Errors in one step don't affect other steps
- Errors in one instance don't affect the system

## Performance Characteristics

### Throughput

- **Single Instance**: ~1000 events/second per step
- **Sharded**: Linear scaling with shard count
- **Bottlenecks**: Usually in adapter implementations (DB, Kafka)

### Latency

- **Step Transition**: 1-10ms (depends on adapter performance)
- **End-to-End**: Sum of step latencies + event propagation time
- **Not optimized for**: Sub-millisecond latencies

### Resource Usage

- **CPU**: Minimal - mostly I/O bound
- **Memory**: ~1MB per workflow definition + adapter overhead
- **Storage**: Linear with number of runs (configurable retention)

## Security Considerations

### Data Protection

- **Encryption at Rest**: Supported by adapter implementations
- **Encryption in Transit**: TLS for all network communication
- **Data Deletion**: Built-in PII scrubbing capabilities

### Access Control

- **API Authentication**: Left to application layer
- **Role-Based Access**: Can be implemented via custom adapters
- **Audit Trail**: All state changes are logged as events

### Network Security

- **No Direct Communication**: Components communicate only through adapters
- **Firewall Friendly**: Standard protocols (HTTP, SQL, Kafka)
- **VPC Support**: All adapters work within private networks

## Monitoring & Observability

### Built-in Metrics

Prometheus metrics for:
- **Throughput**: Events processed per second
- **Latency**: Time between event production and consumption
- **Error Rates**: Failed step executions
- **Queue Depth**: Pending events in each step
- **Role Health**: Active/inactive role assignments

### Distributed Tracing

- **Context Propagation**: Trace context flows through all operations
- **Span Creation**: Each step execution creates a span
- **Custom Attributes**: Workflow name, foreign ID, run ID, status

### Logging

Structured logging with:
- **Correlation IDs**: Track operations across distributed instances
- **Debug Mode**: Verbose logging for troubleshooting
- **Custom Loggers**: Pluggable logging adapters

## Deployment Patterns

### Development

```yaml
# docker-compose.yml
services:
  workflow-app:
    build: .
    environment:
      - WORKFLOW_ADAPTERS=memory
```

### Production - Single Region

```yaml
# kubernetes deployment
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: workflow-app
        env:
        - name: KAFKA_BROKERS
          value: "kafka:9092"
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: url
```

### Production - Multi-Region

```yaml
# Each region has its own deployment
# Shared: Database (with replication)
# Regional: Kafka clusters, role schedulers
# Global: Workflow definitions
```

## Testing Strategies

### Unit Testing

```go
func TestPaymentStep(t *testing.T) {
    run := &workflow.Run[Order, OrderStatus]{
        Object: &Order{Amount: 100.00},
    }

    status, err := processPayment(ctx, run)
    assert.NoError(t, err)
    assert.Equal(t, PaymentProcessed, status)
}
```

### Integration Testing

```go
func TestWorkflowEndToEnd(t *testing.T) {
    wf := NewOrderWorkflow()
    defer wf.Stop()

    wf.Run(ctx)

    runID, _ := wf.Trigger(ctx, "order-123", workflow.WithInitialValue(order))
    run, err := wf.Await(ctx, "order-123", runID, OrderCompleted)

    assert.NoError(t, err)
    assert.Equal(t, OrderCompleted, run.Status)
}
```

### Load Testing

- Use production-like adapters (not memory)
- Test role failover scenarios
- Validate performance under sustained load
- Test error recovery patterns

## Next Steps

- **[Configuration](configuration.md)** - Tuning workflow performance
- **[Monitoring](monitoring.md)** - Production observability
- **[Deployment](deployment.md)** - Production deployment patterns
- **[Advanced Topics](advanced/)** - Deep dives into specific areas