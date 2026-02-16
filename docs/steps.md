# Steps

Steps are the core building blocks of workflows. They define the business logic that processes your data as it flows through different states. This guide covers everything you need to know about creating robust, scalable workflow steps.

## Step Basics

A step is a function that:
- Receives the current Run state
- Executes business logic
- Returns the next status to transition to (or an error)

```go
func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Your business logic here
    if err := chargePaymentMethod(r.Object.PaymentMethod, r.Object.Total); err != nil {
        return PaymentFailed, err
    }

    r.Object.PaymentProcessedAt = time.Now()
    return PaymentProcessed, nil
}
```

### Step Function Signature

```go
type ConsumerFunc[Type any, Status StatusType] func(
    ctx context.Context,
    r *Run[Type, Status]
) (Status, error)
```

**Parameters:**
- `ctx`: Request context with cancellation and deadlines
- `r`: The current workflow run containing your business data

**Return Values:**
- `Status`: The next status to transition to (your enum)
- `error`: If non-nil, the step will be retried

## Adding Steps to Workflows

Use the Builder's `AddStep` method to define steps:

```go
b := workflow.NewBuilder[Order, OrderStatus]("order-processing")

// Define a step that can transition to multiple states
b.AddStep(
    OrderCreated,           // From status
    validateOrder,          // Step function
    OrderValidated,         // Allowed destination status
    OrderRejected,          // Another allowed destination
)

// Chain multiple steps
b.AddStep(OrderValidated, processPayment, PaymentProcessed, PaymentFailed)
b.AddStep(PaymentProcessed, fulfillOrder, OrderFulfilled)
b.AddStep(PaymentFailed, handlePaymentFailure, OrderCancelled, PaymentRetry)
```

### Allowed Destinations

The destinations you specify in `AddStep` are the only statuses your step function can transition to:

```go
b.AddStep(OrderCreated, validateOrder, OrderValidated, OrderRejected)

func validateOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    if r.Object.Items == nil || len(r.Object.Items) == 0 {
        return OrderRejected, nil // ✅ Valid - OrderRejected is allowed
    }

    // return OrderFulfilled, nil // ❌ Would panic - not in allowed destinations

    return OrderValidated, nil // ✅ Valid
}
```

## Data Manipulation

Steps can modify the workflow object directly:

```go
func enrichOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Modify the object directly
    r.Object.ProcessedAt = time.Now()
    r.Object.TrackingNumber = generateTrackingNumber()

    // Add calculated fields
    r.Object.TotalWithTax = r.Object.Subtotal * 1.08

    // Fetch additional data
    customer, err := customerService.GetByID(r.Object.CustomerID)
    if err != nil {
        return 0, err
    }
    r.Object.Customer = customer

    return OrderEnriched, nil
}
```

**Important**: Changes to `r.Object` are automatically persisted when the step completes successfully.

## Error Handling

### Retryable Errors

Return an error to trigger automatic retry with exponential backoff:

```go
func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    err := paymentGateway.Charge(r.Object.PaymentMethod, r.Object.Total)
    if err != nil {
        if isNetworkError(err) || isTemporaryError(err) {
            return 0, err // Will retry automatically
        }

        // Non-retryable error - transition to failure state
        r.Object.PaymentError = err.Error()
        return PaymentFailed, nil
    }

    return PaymentProcessed, nil
}
```

### Error Configuration

Configure retry behavior per step:

```go
b.AddStep(OrderCreated, processPayment, PaymentProcessed, PaymentFailed).
    WithOptions(
        workflow.ErrBackOff(time.Second * 30),    // Wait 30s between retries
        workflow.PauseAfterErrCount(5),           // Pause after 5 consecutive errors
    )
```

### Error Context

Provide helpful error messages:

```go
func validateInventory(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    for _, item := range r.Object.Items {
        available, err := inventory.CheckAvailability(item.ProductID)
        if err != nil {
            return 0, fmt.Errorf("failed to check inventory for product %s: %w", item.ProductID, err)
        }

        if available < item.Quantity {
            r.Object.ValidationErrors = append(r.Object.ValidationErrors,
                fmt.Sprintf("insufficient inventory for product %s: need %d, have %d",
                    item.ProductID, item.Quantity, available))
            return OrderRejected, nil
        }
    }

    return InventoryValidated, nil
}
```

## Conditional Logic

Steps can implement complex branching logic:

```go
func routeOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    switch {
    case r.Object.Total > 1000:
        return OrderRequiresApproval, nil

    case r.Object.IsInternational:
        return OrderRequiresCustomsCheck, nil

    case r.Object.Customer.IsPremium:
        return OrderExpedited, nil

    default:
        return OrderStandard, nil
    }
}

// Define step with multiple possible destinations
b.AddStep(OrderValidated, routeOrder,
    OrderRequiresApproval,
    OrderRequiresCustomsCheck,
    OrderExpedited,
    OrderStandard)
```

## Save and Repeat

Steps can loop by transitioning back to the same state:

```go
func processRetry(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    r.Object.RetryCount++

    if r.Object.RetryCount > 3 {
        return OrderFailed, nil
    }

    if err := attemptProcessing(r.Object); err != nil {
        // Save current state and repeat this step
        return ProcessingRetry, nil
    }

    return OrderCompleted, nil
}

// Allow step to transition back to itself
b.AddStep(ProcessingRetry, processRetry, ProcessingRetry, OrderCompleted, OrderFailed)
```

### Preventing Infinite Loops

Always include termination conditions:

```go
func batchProcessor(ctx context.Context, r *workflow.Run[BatchJob, JobStatus]) (JobStatus, error) {
    // Process one batch
    batch, hasMore := r.Object.GetNextBatch()
    if err := processBatch(batch); err != nil {
        return 0, err
    }

    // Update progress
    r.Object.ProcessedCount += len(batch)
    r.Object.LastProcessedAt = time.Now()

    // Continue if more work to do
    if hasMore {
        return JobBatchProcessing, nil // Repeat this step
    }

    return JobCompleted, nil
}
```

## External Service Integration

### HTTP APIs

```go
func notifyShippingProvider(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    client := &http.Client{Timeout: 30 * time.Second}

    payload := ShippingRequest{
        OrderID: r.Object.ID,
        Address: r.Object.ShippingAddress,
        Items:   r.Object.Items,
    }

    resp, err := client.Post("https://api.shipping.com/orders", "application/json", payload)
    if err != nil {
        return 0, fmt.Errorf("shipping API request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 500 {
        return 0, fmt.Errorf("shipping API server error: %d", resp.StatusCode) // Retry
    }

    if resp.StatusCode >= 400 {
        return OrderShippingFailed, nil // Don't retry client errors
    }

    var response ShippingResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return 0, fmt.Errorf("failed to parse shipping response: %w", err)
    }

    r.Object.TrackingNumber = response.TrackingNumber
    return OrderShipped, nil
}
```

### Database Operations

```go
func updateInventory(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return 0, err
    }
    defer tx.Rollback()

    for _, item := range r.Object.Items {
        _, err := tx.ExecContext(ctx,
            "UPDATE inventory SET quantity = quantity - ? WHERE product_id = ?",
            item.Quantity, item.ProductID)
        if err != nil {
            return 0, fmt.Errorf("failed to update inventory for %s: %w", item.ProductID, err)
        }
    }

    if err := tx.Commit(); err != nil {
        return 0, err
    }

    return InventoryUpdated, nil
}
```

### Idempotency

Make steps idempotent for safe retries:

```go
func chargeCustomer(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Use workflow Run ID as idempotency key
    idempotencyKey := r.RunID

    charge, err := paymentService.CreateCharge(ChargeRequest{
        Amount:         r.Object.Total,
        PaymentMethod:  r.Object.PaymentMethod,
        IdempotencyKey: idempotencyKey, // Prevents duplicate charges
    })
    if err != nil {
        if errors.Is(err, payment.ErrAlreadyProcessed) {
            // Charge already exists - retrieve it
            charge, err = paymentService.GetChargeByIdempotencyKey(idempotencyKey)
            if err != nil {
                return 0, err
            }
        } else {
            return 0, err
        }
    }

    r.Object.ChargeID = charge.ID
    r.Object.ChargedAt = charge.CreatedAt

    return PaymentProcessed, nil
}
```

## Performance Optimization

### Parallel Processing

For high-throughput steps, enable parallel processing:

```go
b.AddStep(OrderCreated, processOrder, OrderProcessed).
    WithOptions(workflow.ParallelCount(10)) // Run 10 parallel instances
```

This creates multiple consumers that process orders concurrently.

### Batching

Process multiple items together:

```go
func batchEmailSender(ctx context.Context, r *workflow.Run[EmailBatch, EmailStatus]) (EmailStatus, error) {
    emails := r.Object.PendingEmails

    // Process in batches of 50
    batchSize := 50
    for i := 0; i < len(emails); i += batchSize {
        end := i + batchSize
        if end > len(emails) {
            end = len(emails)
        }

        batch := emails[i:end]
        if err := emailService.SendBatch(batch); err != nil {
            return 0, err
        }

        r.Object.SentCount += len(batch)
    }

    return EmailsSent, nil
}
```

### Lazy Loading

Only fetch data when needed:

```go
func enrichOrderData(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Only fetch customer data if not already present
    if r.Object.Customer == nil {
        customer, err := customerService.GetByID(r.Object.CustomerID)
        if err != nil {
            return 0, err
        }
        r.Object.Customer = customer
    }

    // Only calculate shipping if order is shippable
    if r.Object.RequiresShipping && r.Object.ShippingCost == 0 {
        cost, err := shippingService.CalculateCost(r.Object)
        if err != nil {
            return 0, err
        }
        r.Object.ShippingCost = cost
    }

    return OrderEnriched, nil
}
```

## Testing Steps

### Unit Testing

Test step functions in isolation:

```go
func TestProcessPayment(t *testing.T) {
    tests := []struct {
        name           string
        order          *Order
        expectedStatus OrderStatus
        expectError    bool
    }{
        {
            name:           "successful payment",
            order:          &Order{Total: 100.00, PaymentMethod: "card_123"},
            expectedStatus: PaymentProcessed,
            expectError:    false,
        },
        {
            name:           "invalid payment method",
            order:          &Order{Total: 100.00, PaymentMethod: "invalid"},
            expectedStatus: PaymentFailed,
            expectError:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            run := &workflow.Run[Order, OrderStatus]{
                Object: tt.order,
            }

            status, err := processPayment(context.Background(), run)

            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedStatus, status)
            }
        })
    }
}
```

### Integration Testing

Test steps with real dependencies:

```go
func TestOrderProcessingIntegration(t *testing.T) {
    wf := NewOrderWorkflow()
    defer wf.Stop()

    ctx := context.Background()
    wf.Run(ctx)

    order := &Order{
        ID:            "order-123",
        CustomerID:    "customer-456",
        Total:         150.00,
        PaymentMethod: "card_789",
    }

    runID, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue(order))
    require.NoError(t, err)

    // Wait for payment processing
    run, err := wf.Await(ctx, order.ID, runID, PaymentProcessed)
    require.NoError(t, err)
    assert.NotNil(t, run.Object.ChargedAt)

    // Wait for completion
    run, err = wf.Await(ctx, order.ID, runID, OrderCompleted)
    require.NoError(t, err)
    assert.Equal(t, OrderCompleted, run.Status)
}
```

### Waiting for Workflow Completion

Workflow provides two methods for waiting for workflows to reach a specific state:

#### WaitForComplete - Simple Completion Waiting

Use `WaitForComplete()` when you just want to wait for the workflow to finish, regardless of which terminal status it reaches:

```go
func TestOrderProcessing(t *testing.T) {
    wf := NewOrderWorkflow()
    defer wf.Stop()

    ctx := context.Background()
    wf.Run(ctx)

    runID, err := wf.Trigger(ctx, "order-123")
    require.NoError(t, err)

    // Wait for any terminal status
    run, err := wf.WaitForComplete(ctx, "order-123", runID)
    require.NoError(t, err)
    
    // Check which terminal status was reached
    switch run.Status {
    case OrderCompleted:
        assert.NotNil(t, run.Object.CompletedAt)
    case OrderCancelled:
        assert.NotNil(t, run.Object.CancelledReason)
    }
}
```

**Benefits:**
- Simpler API - no need to specify the expected status
- Works with workflows that have multiple terminal statuses
- Ideal for most testing scenarios

#### Await - Specific Status Waiting

Use `Await()` when you need to:
- Wait for a specific intermediate status
- Assert that a particular terminal status is reached
- Validate workflow state at specific points

```go
func TestOrderSteps(t *testing.T) {
    wf := NewOrderWorkflow()
    defer wf.Stop()

    ctx := context.Background()
    wf.Run(ctx)

    runID, err := wf.Trigger(ctx, "order-123")
    require.NoError(t, err)

    // Wait for intermediate step
    run, err := wf.Await(ctx, "order-123", runID, PaymentProcessed)
    require.NoError(t, err)
    assert.NotNil(t, run.Object.ChargedAt)
    
    // Verify payment was charged before continuing
    assert.Greater(t, run.Object.ChargedAt, time.Now().Add(-time.Minute))

    // Wait for specific terminal status
    run, err = wf.Await(ctx, "order-123", runID, OrderCompleted)
    require.NoError(t, err)
    assert.Equal(t, OrderCompleted, run.Status)
}
```

**Use Await when:**
- Testing multi-step workflows and need to validate intermediate states
- Ensuring a specific terminal status is reached (not just any completion)
- Debugging workflows by checking state at each step

**Common patterns:**

```go
// Pattern 1: Wait for completion with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
run, err := wf.WaitForComplete(ctx, foreignID, runID)

// Pattern 2: Validate intermediate steps
run, err := wf.Await(ctx, foreignID, runID, IntermediateStatus)
require.NoError(t, err)
// Assert something about run.Object at this intermediate state

// Pattern 3: Poll frequency control
run, err := wf.WaitForComplete(ctx, foreignID, runID, 
    workflow.WithAwaitPollingFrequency(100*time.Millisecond))
```

## Best Practices

### 1. Keep Steps Small and Focused

```go
// ❌ Bad - too much responsibility
func processOrderMegaStep(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Validate order (should be separate step)
    // Check inventory (should be separate step)
    // Process payment (should be separate step)
    // Send confirmation email (should be separate step)
    // Update analytics (should be separate step)
}

// ✅ Good - single responsibility
func validateOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Only validation logic here
}

func checkInventory(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Only inventory checking here
}
```

### 2. Use Meaningful Status Names

```go
// ❌ Bad
const (
    Status1 OrderStatus = iota + 1
    Status2
    Status3
)

// ✅ Good
const (
    OrderStatusUnknown         OrderStatus = 0
    OrderStatusCreated         OrderStatus = 1
    OrderStatusPaymentProcessed OrderStatus = 2
    OrderStatusInventoryReserved OrderStatus = 3
    OrderStatusFulfilled       OrderStatus = 4
)
```

### 3. Handle Context Cancellation

```go
func longRunningStep(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    for _, item := range r.Object.Items {
        select {
        case <-ctx.Done():
            return 0, ctx.Err() // Respect cancellation
        default:
            if err := processItem(item); err != nil {
                return 0, err
            }
        }
    }
    return OrderProcessed, nil
}
```

### 4. Validate State Transitions

```go
func fulfillOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    // Validate we're in the expected state
    if r.Object.ChargedAt.IsZero() {
        return 0, fmt.Errorf("cannot fulfill order that hasn't been charged")
    }

    if r.Object.ShippingAddress == nil {
        return 0, fmt.Errorf("cannot fulfill order without shipping address")
    }

    // Proceed with fulfillment
    return fulfillOrderInternal(r.Object)
}
```

### 5. Use Structured Logging

```go
func processPayment(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
    log.InfoContext(ctx, "processing payment",
        "order_id", r.Object.ID,
        "amount", r.Object.Total,
        "run_id", r.RunID)

    // ... processing logic ...

    log.InfoContext(ctx, "payment processed successfully",
        "order_id", r.Object.ID,
        "charge_id", r.Object.ChargeID)

    return PaymentProcessed, nil
}
```

## Next Steps

- **[Callbacks](callbacks.md)** - Handle external events and webhooks
- **[Timeouts](timeouts.md)** - Add time-based operations to workflows
- **[Configuration](configuration.md)** - Tune step performance and behavior
- **[Examples](examples/)** - See step patterns in real workflows