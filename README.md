<div align="center">
    <img src="./logo/logo.png" style="width: 150px; margin: 30px" alt="Workflow Logo">
    <div  align="center" style="max-width: 750px">
        <a style="padding: 0 5px" href="https://goreportcard.com/report/github.com/luno/workflow"><img src="https://goreportcard.com/badge/github.com/luno/workflow"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=coverage"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=alert_status"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=bugs"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=reliability_rating"/></a><a style="text-decoration:none; padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow" ><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=security_rating"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=sqale_rating"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=vulnerabilities"/></a>
        <a style="padding: 0 5px" href="https://sonarcloud.io/summary/new_code?id=luno_workflow"><img src="https://sonarcloud.io/api/project_badges/measure?project=luno_workflow&metric=duplicated_lines_density"/></a>
    </div>
</div>

# Workflow

Workflow is an event driven workflow that allows for robust, durable, and scalable sequential business logic to
be executed in a deterministic manner.

---
## Features

- **Tech stack agnostic:** Use Kafka, Cassandra, Redis, MongoDB, Postgresql, MySQL, RabbitM, or Reflex - the choice is yours!
- **Graph based (Directed Acyclic Graph - DAG):** Design the workflow by defining small units of work called "Steps".
- **TDD:** Workflow was built using TDD and remains well-supported through a suit of tools.
- **Schedule:** Allows standard cron spec to schedule workflows 
- **Timeouts:** Set either a dynamic or static time for a workflow to wait for. Once the timeout finishes everything continues as it was.
- **Event fusion:** Add event connectors to your workflow to consume external event streams (even if its from a different event streaming platform).  
- **Callbacks:** Allow for manual callbacks from webhooks or manual triggers from consoles to progress the workflow such as approval buttons or third-party webhooks.  
- **Parallel consumers:** Specify how many step consumers should run or specify the default for all consumers. 
- **Consumer management:** Consumer management and graceful shutdown of all processes making sure there is no goroutine leaks!

---

## Installation
To start using workflow you will need to add the workflow module to your project. You can do this by running:
```bash
go get github.com/luno/workflow
```

### Adapters
Some adapters dont come with the core workflow module such as `kafkastreamer`, `reflexstreamer`, `sqlstore`, and `sqltimeout`. If you
 wish to use these you need to add them individually based on your needs or build out your own adapter.

#### Kafka
```bash
go get github.com/luno/workflow/adapters/kafkastreamer
```

#### Reflex
```bash
go get github.com/luno/workflow/adapters/reflexstreamer
```

#### SQL Store
```bash
go get github.com/luno/workflow/adapters/sqlstore
```

#### SQL Timeout
```bash
go get github.com/luno/workflow/adapters/sqltimeout
```
---
## Usage

### Step 1: Define the workflow
```go
package usage

import (
	"context"

	"github.com/luno/workflow"
)

type Step int

func (s Step) String() string {
	switch s {
	case StepOne:
		return "One"
	case StepTwo:
		return "Two"
	case StepThree:
		return "Three"
	default:
		return "Unknown"
	}
}

const (
	StepUnknown Step = 0
	StepOne Step = 1
	StepTwo Step = 2
	StepThree Step = 3
)

type MyType struct {
	Field string
}

func Workflow() *workflow.Workflow[MyType, Step] {
	b := workflow.NewBuilder[MyType, Step]("my workflow name")

	b.AddStep(StepOne, func(ctx context.Context, r *workflow.Run[MyType, Step]) (Step, error) {
		r.Object.Field = "Hello,"
		return StepTwo, nil
	}, StepTwo)

	b.AddStep(StepTwo, func(ctx context.Context, r *workflow.Run[MyType, Step]) (Step, error) {
		r.Object.Field += " world!"
		return StepThree, nil
	}, StepThree)

	return b.Build(...)
}
```
```mermaid
---
title: The above defined workflow creates the below Directed Acyclic Graph
---
stateDiagram-v2
	direction LR
    
	[*]-->One
    One-->Two
    Two-->Three
    Three-->[*]
```
### Step 2: Run the workflow
```go
wf := usage.Workflow()

ctx := context.Background()
wf.Run(ctx)
```
**Stop:** To stop all processes and wait for them to shut down correctly call
```go
wf.Stop()
```
### Step 3: Trigger the workflow
```go
foreignID := "82347982374982374"
runID, err := wf.Trigger(ctx, foreignID, StepOne)
if err != nil {
	...
}
```
**Awaiting results:** If appropriate and desired you can wait for the workflow to complete. Using context timeout (cancellation) is advised.
```go
foreignID := "82347982374982374"
runID, err := wf.Trigger(ctx, foreignID, StepOne)
if err != nil {
	...
}

ctx, cancel := context.WithTimeout(ctx, 10 * time.Second)
defer cancel()

record, err := wf.Await(ctx, foreignID, runID, StepThree)
if err != nil {
	...
}
```

### Detailed examples
Head on over to [./examples](./examples) to get familiar with **callbacks**, **timeouts**, **testing**, **connectors** and
 more about the syntax in depth 😊

---

## Workflow's RunState
RunState is the state of a workflow run and can only exist in one state at any given time. RunState is a
 finite state machine and allows for control over the workflow run. A workflow run is every instance of
 a triggered workflow.
```mermaid
---
title: Diagram the run states of a workflow
---
stateDiagram-v2
	direction LR
    
    Initiated-->Running
    
    Running-->Completed
    Running-->Paused

    Paused-->Running
    
    Running --> Cancelled
    Paused --> Cancelled
    
    state Finished {
        Completed --> RequestedDataDeleted
        Cancelled --> RequestedDataDeleted
            
        DataDeleted-->RequestedDataDeleted
        RequestedDataDeleted-->DataDeleted
    }
```

---

## Configuration Options

This package provides several options to configure the behavior of the workflow process. You can use these options to customize the instance count, polling frequency, error handling, lag settings, and more. Each option is defined as a function that takes a pointer to an `options` struct and modifies it accordingly. Below is a description of each available option:

### `ParallelCount`

```go
func ParallelCount(instances int) Option
```

- **Description:** Defines the number of instances of the workflow process. These instances are distributed consistently, each named to reflect its position (e.g., "consumer-1-of-5"). This helps in managing parallelism in workflow execution.
- **Parameters:**
    - `instances`: The total number of parallel instances to create.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.ParallelCount(5)
)
```

### `PollingFrequency`

```go
func PollingFrequency(d time.Duration) Option
```

- **Description:** Sets the duration at which the workflow process polls for changes. Adjust this to control how frequently the process checks for new events or updates.
- **Parameters:**
    - `d`: The polling frequency as a `time.Duration`.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.PollingFrequency(10 * time.Second)
)
```

### `ErrBackOff`

```go
func ErrBackOff(d time.Duration) Option
```

- **Description:** Defines the duration for which the workflow process will back off after encountering an error. This is useful for managing retries and avoiding rapid repeated failures.
- **Parameters:**
    - `d`: The backoff duration as a `time.Duration`.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.ErrBackOff(5 * time.Minute)
)
```
### `LagAlert`

```go
func LagAlert(d time.Duration) Option
```

- **Description:** Specifies the time threshold before a Prometheus metric switches to true, indicating that the workflow consumer is struggling to keep up. This can signal the need to convert to a parallel consumer.
- **Parameters:**
    - `d`: The duration of the lag alert as a `time.Duration`.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.LagAlert(15 * time.Minute),
)
```
### `ConsumeLag`

```go
func ConsumeLag(d time.Duration) Option
```

- **Description:** Defines the maximum age of events that the consumer will process. Events newer than the specified duration will be held until they are older than the lag period.
- **Parameters:**
    - `d`: The lag duration as a `time.Duration`.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.ConsumeLag(10 * time.Minute),
)
```
### `PauseAfterErrCount`

```go
func PauseAfterErrCount(count int) Option
```

- **Description:** Sets the number of errors allowed before a record is updated to `RunStatePaused`. This mechanism acts similarly to a Dead Letter Queue, preventing further processing of problematic records and allowing for investigation and retry.
- **Parameters:**
    - `count`: The maximum number of errors before pausing.
- **Usage Example:**
```go
b.AddStep(
    StepOne,
    ...,
    StepTwo,
).WithOptions(
    workflow.PauseAfterErrCount(3),
)
```
---

## Glossary

| **Term**         | **Description**                                                                                                                                                                                                       |
|------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Builder**      | A struct type that facilitates the construction of workflows. It provides methods for adding steps, callbacks, timeouts, and connecting workflows.                                                                    |
| **Callback**     | A method in the workflow API that can be used to trigger a callback function for a specified status. It passes data from a reader to the specified callback function.                                                 |
| **Consumer**     | A component that consumes events from an event stream. In this context, it refers to the background consumer goroutines launched by the workflow.                                                                     |
| **EventStreamer**| An interface representing a stream for workflow events. It includes methods for producing and consuming events.                                                                                                       |
| **Graph**        | A representation of the workflow's structure, showing the relationships between different statuses and transitions.                                                                                                   |
| **Producer**     | A component that produces events to an event stream. It is responsible for sending events to the stream.                                                                                                              |
| **Record**  | Is the wire format and representation of a run that can be stored and retrieved. The RecordStore is used for storing and retrieving records.                                                                          |
| **RecordStore**  | An interface representing a store for Record(s). It defines the methods needed for storing and retrieving records. The RecordStore's underlying technology must support transactions in order to prevent dual-writes. |
| **RoleScheduler**| An interface representing a scheduler for roles in the workflow. It is responsible for coordinating the execution of different roles.                                                                                 |
| **Topic**        | A method that generates a topic for producing events in the event streamer based on the workflow name and status.                                                                                                     |
| **Trigger**      | A method in the workflow API that initiates a workflow for a specified foreignID and starting status. It returns a runID and allows for additional configuration options.                                             |
| **WireFormat**   | A format used for serializing and deserializing data for communication between workflow components. It refers to the wire format of the WireRecord.                                                                   |
| **WireRecord**   | A struct representing a record with additional metadata used for communication between workflow components. It can be marshaled to a wire format for storage and transmission.                                        |
