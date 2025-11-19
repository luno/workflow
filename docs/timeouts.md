# Timeouts

Timeouts in the workflow system allow you to execute logic after a specified duration has elapsed. This is essential for implementing delays, SLA monitoring, automatic escalations, and time-based business rules.

## Overview

The timeout system consists of two main components:

1. **TimerFunc**: Determines when a timeout should expire
2. **TimeoutFunc**: Executes when the timeout expires

## Basic Usage

Add a timeout to your workflow using the `AddTimeout` method:

```go
b.AddTimeout(StatusWaiting,
    func(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
        // Set timeout for 1 hour from now
        return now.Add(time.Hour), nil
    },
    func(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (Status, error) {
        // Handle timeout - escalate the order
        r.Object.Priority = "HIGH"
        return StatusEscalated, nil
    },
    StatusEscalated,
)
```

## Timer Functions

Timer functions determine when a timeout should expire. They return a `time.Time` indicating the expiration time.

### Duration-based Timeouts

Use the built-in `DurationTimerFunc` for fixed durations:

```go
// Timeout after 30 minutes
timer := workflow.DurationTimerFunc[Order, Status](30 * time.Minute)

b.AddTimeout(StatusPending, timer, timeoutHandler, StatusExpired)
```

### Time-based Timeouts

Use `TimeTimerFunc` to timeout at a specific time:

```go
// Timeout at end of business day
endOfDay := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
timer := workflow.TimeTimerFunc[Order, Status](endOfDay)

b.AddTimeout(StatusProcessing, timer, timeoutHandler, StatusExpired)
```

### Dynamic Timeouts

Create custom timer functions for dynamic behavior:

```go
func dynamicTimer(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    switch r.Object.Priority {
    case "HIGH":
        return now.Add(15 * time.Minute), nil
    case "MEDIUM":
        return now.Add(1 * time.Hour), nil
    case "LOW":
        return now.Add(24 * time.Hour), nil
    default:
        // Return zero time to skip timeout
        return time.Time{}, nil
    }
}
```

### Conditional Timeouts

Skip timeout creation by returning a zero time:

```go
func conditionalTimer(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    if r.Object.RequiresTimeout {
        return now.Add(time.Hour), nil
    }

    // No timeout needed
    return time.Time{}, nil
}
```

## Timeout Functions

Timeout functions execute when a timeout expires. They can modify the workflow object and transition to a new status.

### Basic Timeout Handler

```go
func handleTimeout(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (Status, error) {
    // Log timeout
    log.Printf("Order %s timed out at %v", r.Object.ID, now)

    // Update object
    r.Object.TimeoutCount++
    r.Object.LastTimeout = now

    // Transition to new status
    return StatusTimedOut, nil
}
```

### Escalation Timeout

```go
func escalateOrder(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (Status, error) {
    // Escalate priority
    if r.Object.Priority == "LOW" {
        r.Object.Priority = "MEDIUM"
    } else if r.Object.Priority == "MEDIUM" {
        r.Object.Priority = "HIGH"
    }

    // Notify management
    err := notifyManagement(ctx, r.Object)
    if err != nil {
        return 0, fmt.Errorf("failed to notify management: %w", err)
    }

    return StatusEscalated, nil
}
```

### Retry with Backoff

```go
func retryWithBackoff(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (Status, error) {
    r.Object.RetryCount++

    if r.Object.RetryCount > 3 {
        // Max retries exceeded
        return StatusFailed, nil
    }

    // Exponential backoff for next retry
    backoff := time.Duration(r.Object.RetryCount) * time.Duration(r.Object.RetryCount) * time.Minute
    r.Object.NextRetryAt = now.Add(backoff)

    return StatusRetrying, nil
}
```

## Configuration Options

Configure timeout behavior using builder options:

### Polling Frequency

```go
b.AddTimeout(StatusWaiting, timer, handler, StatusNext).WithOptions(
    workflow.PollFrequency(30 * time.Second),
)
```

### Error Backoff

```go
b.AddTimeout(StatusWaiting, timer, handler, StatusNext).WithOptions(
    workflow.ErrBackOff(5 * time.Minute),
)
```

### Pause After Error Count

```go
b.AddTimeout(StatusWaiting, timer, handler, StatusNext).WithOptions(
    workflow.PauseAfterErrCount(3),
)
```

## Timeout Store

The workflow system requires a timeout store implementation to persist timeout records:

```go
// Memory store for testing
timeoutStore := memtimeoutstore.New()

// SQL store for production
timeoutStore := sqltimeout.New(db, "timeouts_table")

workflow := b.Build(
    eventStreamer,
    recordStore,
    roleScheduler,
    workflow.WithTimeoutStore(timeoutStore),
)
```

## Timeout Lifecycle

1. **Creation**: When a workflow record reaches a status with timeouts, the auto-inserter creates timeout records
2. **Storage**: Timeout records are persisted with expiration times
3. **Polling**: Timeout pollers continuously check for expired timeouts
4. **Execution**: Expired timeouts trigger their timeout functions
5. **Completion**: Successfully processed timeouts are marked as completed

## Error Handling

### Timer Function Errors

If a timer function returns an error, the timeout is not created:

```go
func errorProneTimer(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    if r.Object.InvalidData {
        return time.Time{}, fmt.Errorf("cannot create timeout: invalid data")
    }

    return now.Add(time.Hour), nil
}
```

### Timeout Function Errors

If a timeout function returns an error, it will be retried with backoff:

```go
func retriableTimeout(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (Status, error) {
    err := externalServiceCall(ctx, r.Object)
    if err != nil {
        // Will be retried
        return 0, fmt.Errorf("external service failed: %w", err)
    }

    return StatusProcessed, nil
}
```

## Best Practices

### Use Clock for Testing

Always use the provided `now` parameter instead of `time.Now()` for testability:

```go
// Good - testable
func timer(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    return now.Add(time.Hour), nil
}

// Bad - hard to test
func timer(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    return time.Now().Add(time.Hour), nil // Don't do this
}
```

### Avoid Long Timeouts

Very long timeouts can accumulate in the timeout store:

```go
// Consider using shorter timeouts with re-scheduling
func reasonableTimeout(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    // Instead of 30 days, use 1 day and re-schedule
    return now.Add(24 * time.Hour), nil
}
```

### Handle Timezone Considerations

Be explicit about timezones for business hour calculations:

```go
func businessHoursTimeout(ctx context.Context, r *workflow.Run[Order, Status], now time.Time) (time.Time, error) {
    // Convert to business timezone
    businessTZ, _ := time.LoadLocation("America/New_York")
    businessTime := now.In(businessTZ)

    // Set timeout for next business day 9 AM
    nextDay := businessTime.Add(24 * time.Hour)
    businessHourStart := time.Date(
        nextDay.Year(), nextDay.Month(), nextDay.Day(),
        9, 0, 0, 0, businessTZ,
    )

    return businessHourStart.UTC(), nil
}
```

## Common Patterns

### SLA Monitoring

```go
func slaTimeout(ctx context.Context, r *workflow.Run[Ticket, Status], now time.Time) (time.Time, error) {
    slaHours := getSLAHours(r.Object.Priority)
    return r.Object.CreatedAt.Add(time.Duration(slaHours) * time.Hour), nil
}

func slaViolation(ctx context.Context, r *workflow.Run[Ticket, Status], now time.Time) (Status, error) {
    r.Object.SLAViolated = true
    r.Object.ViolationTime = now

    // Notify stakeholders
    err := notifySLAViolation(ctx, r.Object)
    if err != nil {
        return 0, err
    }

    return StatusSLAViolated, nil
}
```

### Reminder System

```go
func reminderTimeout(ctx context.Context, r *workflow.Run[Task, Status], now time.Time) (time.Time, error) {
    return r.Object.DueDate.Add(-24 * time.Hour), nil // Remind 1 day before
}

func sendReminder(ctx context.Context, r *workflow.Run[Task, Status], now time.Time) (Status, error) {
    err := sendEmailReminder(ctx, r.Object.AssigneeEmail, r.Object)
    if err != nil {
        return 0, err
    }

    // Don't change status, just send reminder
    return StatusInProgress, nil
}
```

### Circuit Breaker

```go
func circuitBreakerTimeout(ctx context.Context, r *workflow.Run[Request, Status], now time.Time) (time.Time, error) {
    // Reset circuit after 5 minutes
    return now.Add(5 * time.Minute), nil
}

func resetCircuit(ctx context.Context, r *workflow.Run[Request, Status], now time.Time) (Status, error) {
    r.Object.FailureCount = 0
    r.Object.CircuitOpen = false

    return StatusRetryReady, nil
}
```

## Testing Timeouts

Use the clock interface for deterministic testing:

```go
func TestTimeout(t *testing.T) {
    fakeClock := clock.NewFakeClock(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))

    workflow := buildWorkflow(t, workflow.WithClock(fakeClock))

    // Create record
    recordID := insertRecord(t, workflow, StatusWaiting)

    // Advance clock to trigger timeout
    fakeClock.Step(2 * time.Hour)

    // Wait for timeout processing
    time.Sleep(100 * time.Millisecond)

    // Verify timeout was processed
    record := getRecord(t, workflow, recordID)
    assert.Equal(t, StatusTimedOut, record.Status)
}
```

## Monitoring

Monitor timeout processing with metrics:

- `workflow_timeout_created_total`: Number of timeouts created
- `workflow_timeout_processed_total`: Number of timeouts processed
- `workflow_timeout_errors_total`: Number of timeout processing errors
- `workflow_timeout_processing_duration_seconds`: Time to process timeouts

## Limitations

1. **No Parallel Processing**: Timeouts are not processed in parallel
2. **No Sub-second Precision**: Minimum polling frequency is typically 1 second
3. **Clock Dependency**: Relies on system clock for accurate timing
4. **Storage Growth**: Long-running timeouts accumulate in storage