# Connectors

Connectors enable your workflows to integrate with external systems by consuming events from external sources and triggering workflow instances. They provide a bridge between external event streams and your workflow system.

## Overview

Connectors consist of three main components:

1. **ConnectorConstructor**: Creates connector consumers
2. **ConnectorConsumer**: Receives events from external sources
3. **ConnectorFunc**: Processes events and triggers workflows

## Basic Usage

Add a connector to your workflow using the `AddConnector` method:

```go
builder.AddConnector(
    "payment-processor",
    paymentConnector,
    func(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
        // Process the external event
        _, err := api.Trigger(ctx, e.ForeignID,
            workflow.WithInitialValue[Order, Status](&Order{
                ID:              e.ForeignID,
                PaymentProvider: e.Headers["provider"],
                Amount:          parseAmount(e.Headers["amount"]),
                Currency:        e.Headers["currency"],
            }),
        )
        return err
    },
)
```

## ConnectorEvent Structure

External events are represented by the `ConnectorEvent` struct:

```go
type ConnectorEvent struct {
    // ID is a unique ID for the event
    ID string

    // ForeignID refers to the external entity ID
    ForeignID string

    // Type indicates the event type
    Type string

    // Headers store additional metadata
    Headers map[string]string

    // CreatedAt is when the event was produced
    CreatedAt time.Time
}
```

## Implementing Connectors

### ConnectorConstructor

The constructor creates connector consumers for each workflow instance:

```go
type PaymentConnector struct {
    queue PaymentQueue
}

func (c *PaymentConnector) Make(ctx context.Context, consumerName string) (workflow.ConnectorConsumer, error) {
    return &PaymentConsumer{
        queue:        c.queue,
        consumerName: consumerName,
    }, nil
}
```

### ConnectorConsumer

The consumer receives events from external sources:

```go
type PaymentConsumer struct {
    queue        PaymentQueue
    consumerName string
}

func (c *PaymentConsumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
    message, err := c.queue.Receive(ctx, c.consumerName)
    if err != nil {
        return nil, nil, err
    }

    event := &workflow.ConnectorEvent{
        ID:        message.ID,
        ForeignID: message.OrderID,
        Type:      message.EventType,
        Headers: map[string]string{
            "provider": message.Provider,
            "amount":   message.Amount,
            "status":   message.Status,
        },
        CreatedAt: message.Timestamp,
    }

    ack := func() error {
        return c.queue.Acknowledge(message.ID)
    }

    return event, ack, nil
}

func (c *PaymentConsumer) Close() error {
    return c.queue.Close()
}
```

## Event Processing Functions

### Basic Event Processing

```go
func processPaymentEvent(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    switch e.Type {
    case "payment.completed":
        return handlePaymentCompleted(ctx, api, e)
    case "payment.failed":
        return handlePaymentFailed(ctx, api, e)
    case "payment.refunded":
        return handlePaymentRefunded(ctx, api, e)
    default:
        // Skip unknown event types
        return nil
    }
}

func handlePaymentCompleted(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    return api.UpdateStatus(ctx, e.ForeignID, StatusPaymentCompleted)
}
```

### Creating New Workflow Instances

```go
func triggerNewOrder(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    order := &Order{
        ID:          e.ForeignID,
        Source:      e.Headers["source"],
        CustomerID:  e.Headers["customer_id"],
        TotalAmount: parseAmount(e.Headers["total"]),
        Currency:    e.Headers["currency"],
        CreatedAt:   e.CreatedAt,
    }

    _, err := api.Trigger(ctx, e.ForeignID,
        workflow.WithInitialValue[Order, Status](order),
    )
    return err
}
```

### Conditional Processing

```go
func conditionalProcessor(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    // Only process high-value orders
    amount, err := strconv.ParseFloat(e.Headers["amount"], 64)
    if err != nil {
        return fmt.Errorf("invalid amount: %w", err)
    }

    if amount < 1000.00 {
        // Skip low-value orders
        return nil
    }

    return api.UpdateStatus(ctx, e.ForeignID, StatusHighValueOrder)
}
```

## Configuration Options

Configure connector behavior using builder options:

### Parallel Processing

```go
builder.AddConnector(
    "webhook-processor",
    webhookConnector,
    processWebhookEvent,
).WithOptions(
    workflow.ParallelCount(5), // Process 5 events in parallel
)
```

### Error Backoff

```go
builder.AddConnector(
    "payment-processor",
    paymentConnector,
    processPaymentEvent,
).WithOptions(
    workflow.ErrBackOff(30 * time.Second),
)
```

### Lag Configuration

```go
builder.AddConnector(
    "notification-processor",
    notificationConnector,
    processNotificationEvent,
).WithOptions(
    workflow.ConsumeLag(5 * time.Minute),
    workflow.LagAlert(10 * time.Minute),
)
```

## Built-in Connectors

### Memory Connector

For testing and development:

```go
events := []workflow.ConnectorEvent{
    {
        ID:        "1",
        ForeignID: "order-123",
        Type:      "order.created",
        Headers: map[string]string{
            "customer": "john@example.com",
            "amount":   "99.99",
        },
        CreatedAt: time.Now(),
    },
}

connector := memstreamer.NewConnector(events)
```

### Reflex Connector

For database event streaming:

```go
connector := reflexstreamer.NewConnector(
    streamFunc,
    cursorStore,
    reflexstreamer.DefaultReflexTranslator,
)

// Custom translator
translator := func(e *reflex.Event) (*workflow.ConnectorEvent, error) {
    return &workflow.ConnectorEvent{
        ID:        e.ID,
        ForeignID: e.ForeignID,
        Type:      getEventType(e.Type),
        Headers: map[string]string{
            "table": getTableName(e.Type),
            "action": getAction(e.Type),
        },
        CreatedAt: e.Timestamp,
    }, nil
}
```

## Event Filtering and Sharding

### Shard-based Processing

Connectors automatically support sharding for horizontal scaling:

```go
// When using ParallelCount(3), events are distributed across 3 consumers
builder.AddConnector(
    "order-processor",
    orderConnector,
    processOrderEvent,
).WithOptions(
    workflow.ParallelCount(3), // Creates 3 consumers with automatic sharding
)
```

### Custom Filtering

Implement filtering in your connector consumer:

```go
func (c *PaymentConsumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
    for {
        message, err := c.queue.Receive(ctx, c.consumerName)
        if err != nil {
            return nil, nil, err
        }

        // Filter out test events in production
        if c.isProduction && strings.HasPrefix(message.OrderID, "test-") {
            // Acknowledge and skip
            c.queue.Acknowledge(message.ID)
            continue
        }

        // Process the event
        return c.convertToConnectorEvent(message), c.createAck(message), nil
    }
}
```

## Error Handling

### Retry Logic

Connector functions are retried automatically on error:

```go
func retriableProcessor(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    err := callExternalService(ctx, e)
    if err != nil {
        // Will be retried with backoff
        return fmt.Errorf("external service failed: %w", err)
    }

    return api.UpdateStatus(ctx, e.ForeignID, StatusProcessed)
}
```

### Dead Letter Handling

```go
func processWithDeadLetter(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    err := processEvent(ctx, api, e)
    if err != nil {
        // Log to dead letter queue after max retries
        if isMaxRetriesExceeded(err) {
            deadLetterQueue.Send(ctx, e)
            return nil // Don't retry further
        }
        return err
    }
    return nil
}
```

### Validation

```go
func validateAndProcess(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    // Validate required headers
    requiredHeaders := []string{"customer_id", "amount", "currency"}
    for _, header := range requiredHeaders {
        if _, exists := e.Headers[header]; !exists {
            // Invalid event - don't retry
            return fmt.Errorf("missing required header: %s", header)
        }
    }

    // Validate amounts
    amount, err := strconv.ParseFloat(e.Headers["amount"], 64)
    if err != nil || amount <= 0 {
        return fmt.Errorf("invalid amount: %s", e.Headers["amount"])
    }

    return processValidEvent(ctx, api, e)
}
```

## Common Integration Patterns

### Webhook Integration

```go
type WebhookConnector struct {
    server *http.Server
    events chan *workflow.ConnectorEvent
}

func (w *WebhookConnector) handleWebhook(writer http.ResponseWriter, request *http.Request) {
    var payload WebhookPayload
    if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
        http.Error(writer, "Invalid payload", http.StatusBadRequest)
        return
    }

    event := &workflow.ConnectorEvent{
        ID:        payload.ID,
        ForeignID: payload.EntityID,
        Type:      payload.EventType,
        Headers:   payload.Metadata,
        CreatedAt: time.Now(),
    }

    select {
    case w.events <- event:
        writer.WriteHeader(http.StatusOK)
    case <-request.Context().Done():
        writer.WriteHeader(http.StatusRequestTimeout)
    }
}
```

### Message Queue Integration

```go
type KafkaConnector struct {
    brokers []string
    topic   string
}

func (k *KafkaConnector) Make(ctx context.Context, consumerName string) (workflow.ConnectorConsumer, error) {
    consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
        "bootstrap.servers": strings.Join(k.brokers, ","),
        "group.id":          consumerName,
        "auto.offset.reset": "earliest",
    })
    if err != nil {
        return nil, err
    }

    err = consumer.SubscribeTopics([]string{k.topic}, nil)
    if err != nil {
        return nil, err
    }

    return &KafkaConsumer{consumer: consumer}, nil
}

type KafkaConsumer struct {
    consumer *kafka.Consumer
}

func (k *KafkaConsumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
    message, err := k.consumer.ReadMessage(30 * time.Second)
    if err != nil {
        return nil, nil, err
    }

    var event EventPayload
    if err := json.Unmarshal(message.Value, &event); err != nil {
        return nil, nil, err
    }

    connectorEvent := &workflow.ConnectorEvent{
        ID:        string(message.Key),
        ForeignID: event.EntityID,
        Type:      event.Type,
        Headers:   event.Headers,
        CreatedAt: time.Now(),
    }

    ack := func() error {
        _, err := k.consumer.CommitMessage(message)
        return err
    }

    return connectorEvent, ack, nil
}
```

### Database Change Streams

```go
type DatabaseConnector struct {
    db    *sql.DB
    table string
}

func (d *DatabaseConnector) Make(ctx context.Context, consumerName string) (workflow.ConnectorConsumer, error) {
    return &DatabaseConsumer{
        db:           d.db,
        table:        d.table,
        consumerName: consumerName,
        lastID:       d.getLastProcessedID(consumerName),
    }, nil
}

type DatabaseConsumer struct {
    db           *sql.DB
    table        string
    consumerName string
    lastID       int64
}

func (d *DatabaseConsumer) Recv(ctx context.Context) (*workflow.ConnectorEvent, workflow.Ack, error) {
    for {
        rows, err := d.db.QueryContext(ctx,
            "SELECT id, entity_id, event_type, headers, created_at FROM "+d.table+" WHERE id > ? ORDER BY id LIMIT 1",
            d.lastID,
        )
        if err != nil {
            return nil, nil, err
        }
        defer rows.Close()

        if !rows.Next() {
            // No new events, wait and retry
            time.Sleep(time.Second)
            continue
        }

        var event struct {
            ID        int64
            EntityID  string
            EventType string
            Headers   string
            CreatedAt time.Time
        }

        if err := rows.Scan(&event.ID, &event.EntityID, &event.EventType, &event.Headers, &event.CreatedAt); err != nil {
            return nil, nil, err
        }

        var headers map[string]string
        json.Unmarshal([]byte(event.Headers), &headers)

        connectorEvent := &workflow.ConnectorEvent{
            ID:        strconv.FormatInt(event.ID, 10),
            ForeignID: event.EntityID,
            Type:      event.EventType,
            Headers:   headers,
            CreatedAt: event.CreatedAt,
        }

        ack := func() error {
            d.lastID = event.ID
            return d.updateLastProcessedID(d.consumerName, event.ID)
        }

        return connectorEvent, ack, nil
    }
}
```

## Testing Connectors

### Unit Testing

```go
func TestConnectorFunction(t *testing.T) {
    mockAPI := &MockAPI{}

    event := &workflow.ConnectorEvent{
        ID:        "test-1",
        ForeignID: "order-123",
        Type:      "payment.completed",
        Headers: map[string]string{
            "amount":   "99.99",
            "currency": "USD",
        },
        CreatedAt: time.Now(),
    }

    err := processPaymentEvent(context.Background(), mockAPI, event)
    require.NoError(t, err)

    // Verify expected API calls
    assert.Equal(t, "order-123", mockAPI.LastForeignID)
    assert.Equal(t, StatusPaymentCompleted, mockAPI.LastStatus)
}
```

### Integration Testing

```go
func TestConnectorIntegration(t *testing.T) {
    events := []workflow.ConnectorEvent{
        {
            ID:        "1",
            ForeignID: "test-order",
            Type:      "order.created",
            Headers: map[string]string{
                "customer": "test@example.com",
            },
            CreatedAt: time.Now(),
        },
    }

    connector := memstreamer.NewConnector(events)

    workflow := builder.
        AddConnector("test-connector", connector, processOrderEvent).
        Build(eventStreamer, recordStore, roleScheduler)

    // Start workflow
    workflow.Run(context.Background())

    // Wait for processing
    time.Sleep(100 * time.Millisecond)

    // Verify workflow was triggered
    record, err := recordStore.Latest(context.Background(), "test-workflow", "test-order")
    require.NoError(t, err)
    assert.Equal(t, StatusInitiated, record.Status)
}
```

## Best Practices

### Idempotency

Ensure connector functions are idempotent:

```go
func idempotentProcessor(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    // Check if already processed
    if e.Headers["idempotency_key"] != "" {
        exists, err := api.Exists(ctx, e.ForeignID)
        if err != nil {
            return err
        }
        if exists {
            // Already processed, skip
            return nil
        }
    }

    return processNewEvent(ctx, api, e)
}
```

### Resource Management

Properly manage resources in connectors:

```go
func (c *DatabaseConsumer) Close() error {
    // Clean up database connections
    if c.stmt != nil {
        c.stmt.Close()
    }

    // Update cursor position
    return c.updateLastProcessedID(c.consumerName, c.lastID)
}
```

### Monitoring

Add observability to your connectors:

```go
func monitoredProcessor(ctx context.Context, api workflow.API[Order, Status], e *workflow.ConnectorEvent) error {
    start := time.Now()
    defer func() {
        metrics.ConnectorProcessingDuration.
            WithLabelValues("payment-processor", e.Type).
            Observe(time.Since(start).Seconds())
    }()

    err := processPaymentEvent(ctx, api, e)
    if err != nil {
        metrics.ConnectorErrors.
            WithLabelValues("payment-processor", e.Type).
            Inc()
    } else {
        metrics.ConnectorSuccess.
            WithLabelValues("payment-processor", e.Type).
            Inc()
    }

    return err
}
```

## Limitations

1. **Event Ordering**: No guarantee of event processing order across shards
2. **At-least-once Delivery**: Events may be processed multiple times
3. **Memory Usage**: Large events consume memory during processing
4. **Serialization**: ConnectorEvent headers are string-based only