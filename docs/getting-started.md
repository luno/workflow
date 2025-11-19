# Getting Started

This guide will walk you through installing Workflow and creating your first workflow in just a few minutes.

## Installation

Add the Workflow library to your Go project:

```bash
go get github.com/luno/workflow
```

## Your First Workflow

Let's build a simple task processing workflow to understand the core concepts.

### 1. Define Your Types

First, define the states your workflow can be in and the data it processes:

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/luno/workflow"
    "github.com/luno/workflow/adapters/memstreamer"
    "github.com/luno/workflow/adapters/memrecordstore"
    "github.com/luno/workflow/adapters/memrolescheduler"
)

// TaskStatus represents the possible states in our workflow
type TaskStatus int

const (
    TaskStatusUnknown   TaskStatus = 0
    TaskStatusCreated   TaskStatus = 1
    TaskStatusValidated TaskStatus = 2
    TaskStatusProcessed TaskStatus = 3
    TaskStatusCompleted TaskStatus = 4
)

func (s TaskStatus) String() string {
    switch s {
    case TaskStatusCreated:   return "Created"
    case TaskStatusValidated: return "Validated"
    case TaskStatusProcessed: return "Processed"
    case TaskStatusCompleted: return "Completed"
    default:                 return "Unknown"
    }
}

// Task represents the data flowing through our workflow
type Task struct {
    ID          string
    Name        string
    Data        string
    ProcessedAt *time.Time
    Valid       bool
}
```

### 2. Build Your Workflow

Create a workflow with steps that transform your data:

```go
func NewTaskWorkflow() *workflow.Workflow[Task, TaskStatus] {
    b := workflow.NewBuilder[Task, TaskStatus]("task-processor")

    // Step 1: Validate the task
    b.AddStep(TaskStatusCreated, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
        // Validate the task
        if r.Object.Name == "" {
            r.Object.Valid = false
            return TaskStatusValidated, fmt.Errorf("task name cannot be empty")
        }

        r.Object.Valid = true
        fmt.Printf("✓ Validated task: %s\n", r.Object.Name)
        return TaskStatusValidated, nil
    }, TaskStatusValidated)

    // Step 2: Process the task
    b.AddStep(TaskStatusValidated, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
        if !r.Object.Valid {
            return TaskStatusCompleted, nil // Skip processing invalid tasks
        }

        // Simulate some processing work
        time.Sleep(100 * time.Millisecond)

        now := time.Now()
        r.Object.ProcessedAt = &now
        r.Object.Data = fmt.Sprintf("processed-%s", r.Object.Data)

        fmt.Printf("✓ Processed task: %s\n", r.Object.Name)
        return TaskStatusProcessed, nil
    }, TaskStatusProcessed, TaskStatusCompleted)

    // Step 3: Complete the task
    b.AddStep(TaskStatusProcessed, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
        fmt.Printf("✓ Completed task: %s at %s\n", r.Object.Name, r.Object.ProcessedAt.Format(time.RFC3339))
        return TaskStatusCompleted, nil
    }, TaskStatusCompleted)

    // Build with in-memory adapters for simplicity
    return b.Build(
        memstreamer.New(),
        memrecordstore.New(),
        memrolescheduler.New(),
    )
}
```

### 3. Run Your Workflow

Start the workflow engine and process some tasks:

```go
func main() {
    // Create and start the workflow
    wf := NewTaskWorkflow()

    ctx := context.Background()
    wf.Run(ctx)
    defer wf.Stop()

    // Create some tasks
    tasks := []Task{
        {ID: "1", Name: "Process Invoice", Data: "invoice-data"},
        {ID: "2", Name: "Send Email", Data: "email-content"},
        {ID: "3", Name: "", Data: "invalid-task"}, // This will fail validation
    }

    // Process each task
    for _, task := range tasks {
        fmt.Printf("\n🚀 Starting workflow for task: %s\n", task.Name)

        runID, err := wf.Trigger(ctx, task.ID, workflow.WithInitialValue(task))
        if err != nil {
            fmt.Printf("❌ Failed to start workflow: %v\n", err)
            continue
        }

        // Wait for completion (with timeout)
        ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
        run, err := wf.Await(ctx, task.ID, runID, TaskStatusCompleted)
        cancel()

        if err != nil {
            fmt.Printf("❌ Workflow failed or timed out: %v\n", err)
        } else {
            fmt.Printf("✅ Workflow completed! Final state: %+v\n", run.Object)
        }
    }

    fmt.Println("\n🎉 All workflows completed!")
}
```

### 4. Run the Example

Save this code to `main.go` and run:

```bash
go mod init workflow-example
go get github.com/luno/workflow
go run main.go
```

You'll see output like:

```
🚀 Starting workflow for task: Process Invoice
✓ Validated task: Process Invoice
✓ Processed task: Process Invoice
✓ Completed task: Process Invoice at 2024-01-15T10:30:45Z
✅ Workflow completed! Final state: {ID:1 Name:Process Invoice Data:processed-invoice-data ProcessedAt:2024-01-15 10:30:45.123 +0000 UTC Valid:true}

🚀 Starting workflow for task: Send Email
✓ Validated task: Send Email
✓ Processed task: Send Email
✓ Completed task: Send Email at 2024-01-15T10:30:45Z
✅ Workflow completed! Final state: {ID:2 Name:Send Email Data:processed-email-content ProcessedAt:2024-01-15 10:30:45.234 +0000 UTC Valid:true}

🚀 Starting workflow for task:
❌ Workflow failed or timed out: task name cannot be empty

🎉 All workflows completed!
```

## Key Concepts Demonstrated

1. **Type Safety**: Your workflow is fully typed with `Task` and `TaskStatus`
2. **State Transitions**: Each step returns the next status to transition to
3. **Data Transformation**: The `Task` object is modified as it flows through steps
4. **Error Handling**: Invalid tasks are handled gracefully
5. **Concurrency**: Multiple workflow instances can run simultaneously

## Next Steps

Now that you have a basic workflow running, explore these topics:

- **[Core Concepts](concepts.md)** - Understand Runs, Events, and State Machines
- **[Callbacks](callbacks.md)** - Handle external events and webhooks
- **[Timeouts](timeouts.md)** - Add time-based operations
- **[Production Deployment](deployment.md)** - Move beyond in-memory adapters
- **[Examples](examples/)** - See real-world workflow patterns

## Common Patterns

### Error Recovery

```go
b.AddStep(TaskValidated, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
    if err := processTask(r.Object); err != nil {
        if isRetryableError(err) {
            return 0, err // Will retry with backoff
        }
        return TaskCompleted, nil // Skip to completion for non-retryable errors
    }
    return TaskProcessed, nil
}).WithOptions(
    workflow.PauseAfterErrCount(3), // Pause after 3 consecutive errors
    workflow.ErrBackOff(time.Second * 5), // Wait 5 seconds between retries
)
```

### Conditional Flows

```go
b.AddStep(TaskValidated, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
    if r.Object.Priority == "high" {
        return TaskExpedited, nil
    }
    return TaskProcessed, nil
}, TaskExpedited, TaskProcessed)
```

### Saving and Repeating

```go
b.AddStep(TaskProcessed, func(ctx context.Context, r *workflow.Run[Task, TaskStatus]) (TaskStatus, error) {
    r.Object.ProcessCount++

    if r.Object.ProcessCount < 3 {
        return TaskProcessed, nil // Repeat this step
    }
    return TaskCompleted, nil
}, TaskProcessed, TaskCompleted)
```

You're now ready to build powerful, type-safe workflows with Workflow!