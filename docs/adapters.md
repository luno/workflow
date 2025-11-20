# Adapters

Adapters make Workflow infrastructure-agnostic by providing standardized interfaces for different technology stacks. This guide explains how adapters work, which ones are available, and how to choose the right combination for your needs.

## Adapter Architecture

Workflow uses the adapter pattern to decouple core workflow logic from infrastructure concerns. Each adapter type serves a specific purpose:

```
┌─────────────────────────────────────────────────────────────┐
│                    Workflow Core                            │
├─────────────────────────────────────────────────────────────┤
│  EventStreamer  │  RecordStore  │ RoleScheduler │ TimeoutStore │
│     Interface   │   Interface   │   Interface   │   Interface  │
├─────────────────┼───────────────┼───────────────┼──────────────┤
│ • Kafka         │ • PostgreSQL  │ • Rink        │ • SQL        │
│ • Reflex        │ • MySQL       │ • etcd        │ • Redis      │
│ • Memory        │ • Memory      │ • Memory      │ • Memory     │
└─────────────────┴───────────────┴───────────────┴──────────────┘
```

## Core Adapter Types

### EventStreamer

**Purpose**: Publish and consume workflow events for step coordination.

**Interface**:
```go
type EventStreamer interface {
    NewProducer() (Producer, error)
    NewConsumer(name string) (Consumer, error)
}

type Producer interface {
    Produce(ctx context.Context, event OutboxEvent) error
    Close() error
}

type Consumer interface {
    Consume(ctx context.Context, fn func(*Event, Ack) error) error
    Close() error
}
```

**Available Adapters**:

| Adapter | Use Case | Install |
|---------|----------|---------|
| **kafkastreamer** | Production event streaming | `go get github.com/luno/workflow/adapters/kafkastreamer` |
| **reflexstreamer** | Luno's Reflex event sourcing | `go get github.com/luno/workflow/adapters/reflexstreamer` |
| **memstreamer** | Development and testing | Built-in |

**Example**:
```go
// Kafka for production
kafkaConfig := sarama.NewConfig()
kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
streamer := kafkastreamer.New([]string{"kafka:9092"}, kafkaConfig)

// Memory for development
streamer := memstreamer.New()
```

### RecordStore

**Purpose**: Persist workflow run state with transactional guarantees.

**Interface**:
```go
type RecordStore interface {
    Store(ctx context.Context, record *Record) error
    Lookup(ctx context.Context, runID string) (*Record, error)
    Latest(ctx context.Context, workflowName, foreignID string) (*Record, error)
    List(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error)

    // Outbox pattern support
    ListOutboxEvents(ctx context.Context, workflowName string, limit int64) ([]OutboxEvent, error)
    DeleteOutboxEvent(ctx context.Context, id string) error
}
```

**Available Adapters**:

| Adapter | Use Case | Install |
|---------|----------|---------|
| **sqlstore** | Production with SQL databases | `go get github.com/luno/workflow/adapters/sqlstore` |
| **memrecordstore** | Development and testing | Built-in |

**Requirements**:
- **ACID Transactions**: Required for transactional outbox pattern
- **Query Support**: Must support filtering, sorting, and pagination
- **Schema Management**: Must handle workflow schema evolution

**Example**:
```go
// PostgreSQL for production
db, err := sql.Open("postgres", "postgres://user:pass@host/db")
store := sqlstore.New(db, "workflow_records", "workflow_outbox")

// Memory for development
store := memrecordstore.New()
```

### RoleScheduler

**Purpose**: Coordinate distributed execution ensuring only one instance of each role runs at a time.

**Interface**:
```go
type RoleScheduler interface {
    Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error)
}
```

**Available Adapters**:

| Adapter | Use Case | Install |
|---------|----------|---------|
| **rinkrolescheduler** | Production distributed coordination | `go get github.com/luno/workflow/adapters/rinkrolescheduler` |
| **memrolescheduler** | Single-instance development | Built-in |

**Example**:
```go
// Rink for production distributed systems
rinkConfig := rink.Config{
    Endpoints: []string{"rink-1:8080", "rink-2:8080"},
}
scheduler := rinkrolescheduler.New(rinkConfig)

// Memory for single instance
scheduler := memrolescheduler.New()
```

### TimeoutStore (Optional)

**Purpose**: Schedule durable timeouts that survive process restarts.

**Interface**:
```go
type TimeoutStore interface {
    Store(ctx context.Context, timeout Timeout) error
    List(ctx context.Context, workflowName string, status Status) ([]Timeout, error)
    Complete(ctx context.Context, id string) error
}
```

**Available Adapters**:

| Adapter | Use Case | Install |
|---------|----------|---------|
| **sqltimeout** | Production durable timeouts | `go get github.com/luno/workflow/adapters/sqltimeout` |
| **memtimeoutstore** | Development and testing | Built-in |

**Example**:
```go
// SQL for production
timeoutStore := sqltimeout.New(db)

// Built with timeout support
wf := b.Build(
    eventStreamer, recordStore, roleScheduler,
    workflow.WithTimeoutStore(timeoutStore),
)
```

## Deployment Patterns

### Development

**Goal**: Fast feedback, easy debugging, minimal setup.

```go
func NewDevelopmentWorkflow() *workflow.Workflow[Order, OrderStatus] {
    return b.Build(
        memstreamer.New(),
        memrecordstore.New(),
        memrolescheduler.New(),
        // No timeout store needed for development
    )
}
```

**Characteristics**:
- ✅ Zero infrastructure dependencies
- ✅ Fast startup/teardown
- ✅ Perfect for unit tests
- ❌ No persistence across restarts
- ❌ Single instance only

### Staging

**Goal**: Production-like environment for integration testing.

```go
func NewStagingWorkflow() *workflow.Workflow[Order, OrderStatus] {
    db := setupDatabase()

    return b.Build(
        kafkastreamer.New(kafkaBrokers, kafkaConfig),
        sqlstore.New(db, "workflow_records", "workflow_outbox"),
        rinkrolescheduler.New(rinkConfig),
        workflow.WithTimeoutStore(sqltimeout.New(db)),
    )
}
```

**Characteristics**:
- ✅ Full production adapters
- ✅ Persistent storage
- ✅ Multi-instance testing
- ⚠️ Shared infrastructure with other services

### Production

**Goal**: Maximum reliability, scalability, and observability.

```go
func NewProductionWorkflow() *workflow.Workflow[Order, OrderStatus] {
    // Production database with connection pooling
    db := setupProductionDB()

    // Kafka with optimal configuration
    kafkaConfig := &sarama.Config{
        Producer.RequiredAcks: sarama.WaitForAll,
        Producer.Retry.Max: 5,
        Consumer.Group.Rebalance.Strategy: sarama.BalanceStrategyRoundRobin,
    }

    return b.Build(
        kafkastreamer.New(kafkaBrokers, kafkaConfig),
        sqlstore.New(db, "workflow_records", "workflow_outbox"),
        rinkrolescheduler.New(rinkConfig),
        workflow.WithTimeoutStore(sqltimeout.New(db)),
        workflow.WithDefaultOptions(
            workflow.ParallelCount(5),
            workflow.ErrBackOff(time.Minute),
            workflow.PauseAfterErrCount(3),
        ),
    )
}
```

## Adapter Testing

All adapter implementations should be tested using the provided adapter test suites:

### EventStreamer Testing

```go
func TestMyEventStreamer(t *testing.T) {
    streamer := myeventstreamer.New(config)
    adaptertest.TestEventStreamer(t, streamer)
}
```

### RecordStore Testing

```go
func TestMyRecordStore(t *testing.T) {
    store := myrecordstore.New(config)
    adaptertest.RunRecordStoreTest(t, store)
}
```

### RoleScheduler Testing

```go
func TestMyRoleScheduler(t *testing.T) {
    scheduler := myrolescheduler.New(config)
    adaptertest.RunRoleSchedulerTest(t, scheduler)
}
```

## Building Custom Adapters

### Custom EventStreamer

```go
type MyEventStreamer struct {
    config Config
}

func (s *MyEventStreamer) NewProducer() (workflow.Producer, error) {
    return &MyProducer{client: s.client}, nil
}

func (s *MyEventStreamer) NewConsumer(name string) (workflow.Consumer, error) {
    return &MyConsumer{
        client:    s.client,
        groupName: name,
    }, nil
}

type MyProducer struct {
    client MyClient
}

func (p *MyProducer) Produce(ctx context.Context, event workflow.OutboxEvent) error {
    return p.client.Publish(ctx, event.Topic, event.Data)
}

func (p *MyProducer) Close() error {
    return p.client.Close()
}

type MyConsumer struct {
    client    MyClient
    groupName string
}

func (c *MyConsumer) Consume(ctx context.Context, fn func(*workflow.Event, workflow.Ack) error) error {
    return c.client.Subscribe(ctx, c.groupName, func(msg Message) error {
        event := &workflow.Event{
            ID:        msg.ID,
            ForeignID: msg.ForeignID,
            Type:      msg.Type,
            // ... map other fields
        }

        ack := &MyAck{msg: msg}
        return fn(event, ack)
    })
}
```

### Custom RecordStore

```go
type MyRecordStore struct {
    db MyDatabase
}

func (s *MyRecordStore) Store(ctx context.Context, record *workflow.Record) error {
    tx, err := s.db.BeginTx(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Store the record
    if err := s.storeRecord(tx, record); err != nil {
        return err
    }

    // Store outbox events
    if err := s.storeOutboxEvents(tx, record.OutboxEvents); err != nil {
        return err
    }

    return tx.Commit()
}

func (s *MyRecordStore) Lookup(ctx context.Context, runID string) (*workflow.Record, error) {
    // Query record by run ID
    row := s.db.QueryRowContext(ctx, "SELECT ... FROM records WHERE run_id = ?", runID)
    return s.scanRecord(row)
}

// Implement other interface methods...
```

## Performance Tuning

### Kafka Configuration

```go
kafkaConfig := &sarama.Config{
    // Producer settings
    Producer.RequiredAcks:        sarama.WaitForAll,  // Durability
    Producer.Retry.Max:           5,                   // Retries
    Producer.Flush.Frequency:     100 * time.Millisecond, // Batching
    Producer.Flush.Messages:      100,                // Batch size
    Producer.Compression:         sarama.CompressionSnappy, // Compression

    // Consumer settings
    Consumer.Offsets.Initial:     sarama.OffsetOldest, // Start from beginning
    Consumer.Fetch.Min:           1024,               // Min fetch size
    Consumer.Fetch.Max:           1024 * 1024,        // Max fetch size
    Consumer.Group.Heartbeat.Interval: 3 * time.Second, // Heartbeat
    Consumer.Group.Session.Timeout:    10 * time.Second, // Session timeout
}
```

### Database Optimization

```sql
-- Indexes for workflow_records
CREATE INDEX idx_workflow_records_workflow_foreign ON workflow_records(workflow_name, foreign_id);
CREATE INDEX idx_workflow_records_status ON workflow_records(workflow_name, status);
CREATE INDEX idx_workflow_records_run_state ON workflow_records(run_state);
CREATE INDEX idx_workflow_records_updated_at ON workflow_records(updated_at);

-- Indexes for workflow_outbox
CREATE INDEX idx_workflow_outbox_workflow_created ON workflow_outbox(workflow_name, created_at);

-- Connection pool settings
max_connections = 100
shared_buffers = '256MB'
effective_cache_size = '1GB'
```

### Memory Management

```go
// Configure workflow options for memory efficiency
workflow.WithDefaultOptions(
    workflow.ParallelCount(5),                     // Don't over-parallelize
    workflow.PollingFrequency(500*time.Millisecond), // Reduce polling frequency
    workflow.ErrBackOff(time.Minute),             // Longer backoff reduces load
)
```

## Monitoring Adapters

Some adapters provide additional monitoring capabilities:

### WebUI Adapter

```go
import "github.com/luno/workflow/adapters/webui"

// Add HTTP handlers for workflow monitoring
http.Handle("/", webui.HomeHandlerFunc(webui.Paths{
    List:       "/api/list",
    ObjectData: "/api/object",
}))
http.HandleFunc("/api/list", webui.ListHandlerFunc(recordStore))
http.HandleFunc("/api/object", webui.ObjectDataHandlerFunc(recordStore))
```

### Logging Adapter

```go
import "github.com/luno/workflow/adapters/jlog"

// Use structured logging
logger := jlog.New()
wf := b.Build(
    eventStreamer, recordStore, roleScheduler,
    workflow.WithLogger(logger),
)
```

## Migration Between Adapters

### Development to Production

1. **Replace adapters** in build configuration
2. **Migrate data** if needed (usually not, since development uses memory)
3. **Update configuration** for production settings
4. **Test thoroughly** with production-like load

### Changing Event Streamers

1. **Deploy new version** with new adapter
2. **Let existing events drain** from old system
3. **Switch traffic** to new system
4. **Decommission old system**

### Database Migration

1. **Schema changes**: Use migration scripts
2. **Data migration**: Export/import if changing database types
3. **Zero-downtime**: Use blue/green deployment pattern

## Best Practices

1. **Use production adapters in staging**: Catch integration issues early
2. **Test adapter combinations**: Some combinations may have unexpected behavior
3. **Monitor adapter performance**: Each adapter adds latency and failure points
4. **Keep adapters updated**: Security and performance improvements
5. **Implement health checks**: Verify adapter connectivity and performance
6. **Plan for failure**: What happens if an adapter becomes unavailable?

Adapters are the foundation of Workflow's flexibility. Choose the right combination for your needs and scale them as your requirements grow.

## Next Steps

- **[Configuration](configuration.md)** - Tune adapter and workflow performance
- **[Deployment](deployment.md)** - Production deployment patterns
- **[Monitoring](monitoring.md)** - Monitor adapter health and performance