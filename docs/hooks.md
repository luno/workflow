# Hooks

Hooks provide a way to execute custom logic when workflow instances reach specific lifecycle states. They operate asynchronously and allow you to implement cross-cutting concerns like notifications, logging, metrics collection, and integration with external systems.

## Overview

The workflow system provides three built-in lifecycle hooks:

1. **OnComplete**: Triggered when a workflow completes successfully
2. **OnCancel**: Triggered when a workflow is cancelled
3. **OnPause**: Triggered when a workflow is paused

Hooks consume events from a special run state change topic and execute in separate consumers for reliability and scalability.

## Basic Usage

Define hooks when building your workflow:

```go
builder := workflow.NewBuilder[Order, Status]("order-processing")

// Add workflow steps...

builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Send completion notification
    return notifyOrderComplete(ctx, record.Object)
})

builder.OnCancel(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Handle cancellation cleanup
    return cleanupCancelledOrder(ctx, record.Object)
})

builder.OnPause(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Log pause event
    return logOrderPause(ctx, record)
})

workflow := builder.Build(eventStreamer, recordStore, roleScheduler)
```

## Hook Function Signature

All hooks use the same function signature:

```go
type RunStateChangeHookFunc[Type any, Status StatusType] func(
    ctx context.Context,
    record *TypedRecord[Type, Status],
) error
```

The `TypedRecord` provides access to:
- **Record**: Raw workflow record with metadata
- **Status**: Current typed status
- **Object**: Typed workflow data

## OnComplete Hook

Triggered when a workflow reaches a terminal status and transitions to `RunStateCompleted`:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    log.Printf("Order %s completed at status %s", record.ForeignID, record.Status.String())

    // Send completion email
    err := emailService.SendOrderComplete(ctx, EmailRequest{
        To:      record.Object.CustomerEmail,
        OrderID: record.Object.ID,
        Total:   record.Object.Total,
        Items:   record.Object.Items,
    })
    if err != nil {
        return fmt.Errorf("failed to send completion email: %w", err)
    }

    // Update analytics
    analytics.TrackOrderComplete(ctx, record.Object.ID, record.Object.Total)

    // Trigger downstream workflows
    _, err = payoutWorkflow.Trigger(ctx, record.Object.MerchantID,
        workflow.WithInitialValue[Payout, PayoutStatus](&Payout{
            OrderID: record.Object.ID,
            Amount:  calculateMerchantPayout(record.Object.Total),
        }),
    )
    return err
})
```

## OnCancel Hook

Triggered when a workflow is explicitly cancelled and transitions to `RunStateCancelled`:

```go
builder.OnCancel(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    log.Printf("Order %s cancelled in status %s", record.ForeignID, record.Status.String())

    // Reverse any payments
    if record.Object.PaymentID != "" {
        err := paymentService.Refund(ctx, record.Object.PaymentID, "Order cancelled")
        if err != nil {
            return fmt.Errorf("failed to refund payment: %w", err)
        }
    }

    // Release inventory
    for _, item := range record.Object.Items {
        err := inventoryService.ReleaseReservation(ctx, item.SKU, item.Quantity)
        if err != nil {
            log.Printf("Warning: failed to release inventory for %s: %v", item.SKU, err)
        }
    }

    // Notify customer
    return notificationService.SendCancellation(ctx, record.Object.CustomerEmail, record.Object.ID)
})
```

## OnPause Hook

Triggered when a workflow is paused due to errors and transitions to `RunStatePaused`:

```go
builder.OnPause(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    log.Printf("Order %s paused at status %s", record.ForeignID, record.Status.String())

    // Create support ticket
    ticket, err := supportSystem.CreateTicket(ctx, SupportTicket{
        Title:       fmt.Sprintf("Order %s requires attention", record.Object.ID),
        Description: fmt.Sprintf("Order paused at status %s", record.Status.String()),
        Priority:    "HIGH",
        Category:    "WORKFLOW_ERROR",
        OrderID:     record.Object.ID,
        CustomerID:  record.Object.CustomerID,
    })
    if err != nil {
        return fmt.Errorf("failed to create support ticket: %w", err)
    }

    // Alert operations team
    return alerting.SendAlert(ctx, Alert{
        Type:        "WORKFLOW_PAUSED",
        Severity:    "WARNING",
        WorkflowID:  record.ForeignID,
        Message:     fmt.Sprintf("Order %s requires manual intervention", record.Object.ID),
        TicketID:    ticket.ID,
    })
})
```

## Error Handling

### Retries and Backoff

Hook errors are automatically retried with exponential backoff:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // This will be retried if it fails
    err := externalService.NotifyCompletion(ctx, record.Object.ID)
    if err != nil {
        // Log the error but let it retry
        log.Printf("Failed to notify external service: %v", err)
        return err
    }

    return nil
})
```

### Non-Retriable Errors

Skip retries by returning a non-retriable error:

```go
import "github.com/luno/workflow/internal/errorsinternal"

builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    if record.Object.CustomerEmail == "" {
        // Don't retry for missing email
        return errorsinternal.NewNonRetriableError(errors.New("customer email is required"))
    }

    return sendEmail(ctx, record.Object.CustomerEmail)
})
```

### Error Isolation

Hook errors don't affect the main workflow. Each hook runs in isolation:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Even if this fails, other hooks and the workflow complete successfully
    err := unreliableExternalService.Notify(ctx, record.Object.ID)
    if err != nil {
        // Log and potentially alert, but don't fail the workflow
        log.Printf("External notification failed: %v", err)
        return err // Will be retried independently
    }

    return nil
})
```

## Common Patterns

### Notifications

```go
type NotificationHook[T any, S StatusType] struct {
    emailService EmailService
    smsService   SMSService
}

func (n *NotificationHook[T, S]) OnComplete() workflow.RunStateChangeHookFunc[T, S] {
    return func(ctx context.Context, record *workflow.TypedRecord[T, S]) error {
        // Type-specific notification logic
        switch record.Status.String() {
        case "ORDER_COMPLETED":
            return n.sendOrderCompleteNotification(ctx, record)
        case "SUBSCRIPTION_ACTIVATED":
            return n.sendSubscriptionNotification(ctx, record)
        default:
            return n.sendGenericNotification(ctx, record)
        }
    }
}
```

### Metrics Collection

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Record completion metrics
    duration := record.UpdatedAt.Sub(record.CreatedAt)

    metrics.WorkflowDuration.
        WithLabelValues("order-processing", record.Status.String()).
        Observe(duration.Seconds())

    metrics.WorkflowCompleted.
        WithLabelValues("order-processing", record.Status.String()).
        Inc()

    // Record business metrics
    if order := record.Object; order != nil {
        metrics.OrderValue.Observe(float64(order.Total))
        metrics.OrderItems.Observe(float64(len(order.Items)))
    }

    return nil
})
```

### Audit Logging

```go
type AuditLogger struct {
    logger Logger
}

func (a *AuditLogger) LogWorkflowEvent(eventType string) workflow.RunStateChangeHookFunc[Order, Status] {
    return func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
        auditEvent := AuditEvent{
            EventType:    eventType,
            WorkflowName: record.WorkflowName,
            ForeignID:    record.ForeignID,
            RunID:        record.RunID,
            Status:       record.Status.String(),
            Timestamp:    time.Now(),
            UserID:       getUserFromContext(ctx),
            Metadata: map[string]interface{}{
                "order_value": record.Object.Total,
                "customer_id": record.Object.CustomerID,
            },
        }

        return a.logger.LogAuditEvent(ctx, auditEvent)
    }
}

// Usage
auditLogger := &AuditLogger{logger: auditService}
builder.OnComplete(auditLogger.LogWorkflowEvent("WORKFLOW_COMPLETED"))
builder.OnCancel(auditLogger.LogWorkflowEvent("WORKFLOW_CANCELLED"))
builder.OnPause(auditLogger.LogWorkflowEvent("WORKFLOW_PAUSED"))
```

### External System Integration

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Sync to external CRM
    err := crmClient.UpdateCustomerOrder(ctx, CRMOrderUpdate{
        CustomerID: record.Object.CustomerID,
        OrderID:    record.Object.ID,
        Status:     "COMPLETED",
        Total:      record.Object.Total,
        CompletedAt: record.UpdatedAt,
    })
    if err != nil {
        return fmt.Errorf("failed to sync to CRM: %w", err)
    }

    // Update data warehouse
    err = dataWarehouse.RecordOrderCompletion(ctx, WarehouseOrderEvent{
        OrderID:         record.Object.ID,
        CompletionTime:  record.UpdatedAt,
        ProcessingTime:  record.UpdatedAt.Sub(record.CreatedAt),
        FinalStatus:     record.Status.String(),
    })
    if err != nil {
        return fmt.Errorf("failed to update data warehouse: %w", err)
    }

    return nil
})
```

### Conditional Logic

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    order := record.Object

    // Different actions based on order type
    switch {
    case order.Total > 1000:
        return handleHighValueOrder(ctx, order)
    case order.IsSubscription:
        return handleSubscriptionOrder(ctx, order)
    case order.CustomerType == "VIP":
        return handleVIPOrder(ctx, order)
    default:
        return handleStandardOrder(ctx, order)
    }
})

func handleHighValueOrder(ctx context.Context, order *Order) error {
    // Send to fraud review
    err := fraudService.ReviewHighValueOrder(ctx, order.ID)
    if err != nil {
        return err
    }

    // Notify account manager
    return notifyAccountManager(ctx, order.CustomerID, order.ID)
}
```

## Hook Architecture

### Event Flow

1. Workflow reaches terminal state (completed) or is paused/cancelled
2. Event is published to the run state change topic
3. Hook consumers receive and process events asynchronously
4. Each hook runs independently with its own retry logic

### Topic Structure

Hooks consume from a special topic:
```
{workflow-name}-run-state-changes
```

This topic receives events when workflows transition to:
- `RunStateCompleted`
- `RunStateCancelled`
- `RunStatePaused`
- `RunStateDataDeleted`

### Consumer Roles

Each hook creates its own consumer with a role like:
```
{workflow-name}-{run-state}-run-state-change-hook-consumer
```

## Testing Hooks

### Unit Testing

```go
func TestOrderCompleteHook(t *testing.T) {
    mockEmail := &MockEmailService{}
    mockAnalytics := &MockAnalytics{}

    hook := func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
        return handleOrderComplete(ctx, record, mockEmail, mockAnalytics)
    }

    record := &workflow.TypedRecord[Order, Status]{
        Record: workflow.Record{
            ForeignID: "order-123",
            Status:    int(StatusCompleted),
        },
        Status: StatusCompleted,
        Object: &Order{
            ID:            "order-123",
            CustomerEmail: "customer@example.com",
            Total:         99.99,
        },
    }

    err := hook(context.Background(), record)
    require.NoError(t, err)

    assert.True(t, mockEmail.WasCalled("SendOrderComplete"))
    assert.True(t, mockAnalytics.WasCalled("TrackOrderComplete"))
}
```

### Integration Testing

```go
func TestHookIntegration(t *testing.T) {
    var completionCalled bool

    workflow := builder.
        AddStep(StatusStarted, func(ctx context.Context, r *workflow.Run[Order, Status]) (Status, error) {
            return StatusCompleted, nil
        }, StatusCompleted).
        OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
            completionCalled = true
            return nil
        }).
        Build(eventStreamer, recordStore, roleScheduler)

    // Start workflow
    workflow.Run(context.Background())

    // Trigger workflow instance
    recordID := triggerWorkflow(t, workflow, "test-order")

    // Wait for completion
    awaitWorkflowCompletion(t, workflow, recordID)

    // Wait for hook processing
    time.Sleep(100 * time.Millisecond)

    assert.True(t, completionCalled, "OnComplete hook should have been called")
}
```

## Best Practices

### Idempotency

Make hooks idempotent to handle duplicate events:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    // Check if already processed
    exists, err := emailService.WasNotificationSent(ctx, record.Object.ID)
    if err != nil {
        return err
    }
    if exists {
        return nil // Already processed
    }

    return emailService.SendOrderComplete(ctx, record.Object.CustomerEmail, record.Object.ID)
})
```

### Timeout Handling

Use context timeouts for external calls:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    return externalService.NotifyCompletion(ctx, record.Object.ID)
})
```

### Resource Management

Clean up resources properly:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    client := createExternalClient()
    defer client.Close()

    return client.SendNotification(ctx, record.Object.ID)
})
```

### Structured Logging

Use structured logging for observability:

```go
builder.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[Order, Status]) error {
    logger := log.With(
        "workflow", record.WorkflowName,
        "foreign_id", record.ForeignID,
        "run_id", record.RunID,
        "status", record.Status.String(),
    )

    logger.Info("Processing completion hook")

    err := processCompletion(ctx, record)
    if err != nil {
        logger.Error("Hook processing failed", "error", err)
        return err
    }

    logger.Info("Hook processing completed")
    return nil
})
```

## Monitoring

Monitor hook execution with metrics:

- `workflow_hook_executions_total`: Number of hook executions
- `workflow_hook_errors_total`: Number of hook errors
- `workflow_hook_duration_seconds`: Hook execution duration

## Limitations

1. **Eventual Consistency**: Hooks are asynchronous and may be delayed
2. **No Ordering Guarantees**: Multiple hooks may execute in any order
3. **No Return Values**: Hooks cannot affect the workflow outcome
4. **Memory Usage**: Hook consumers maintain their own event processing state
5. **Error Isolation**: Hook errors don't fail the workflow but may accumulate