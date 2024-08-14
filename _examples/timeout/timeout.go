package timeout

import (
	"context"
	"time"

	"github.com/luno/workflow"
	"k8s.io/utils/clock"
)

type Status int

var (
	StatusUnknown            Status = 0
	StatusStarted            Status = 1
	StatusReadTheDocs        Status = 2
	StatusFollowedTheExample Status = 3
	StatusCreatedAFunExample Status = 4
)

func (s Status) String() string {
	switch s {
	case StatusStarted:
		return "Started"
	case StatusReadTheDocs:
		return "Read the docs"
	case StatusFollowedTheExample:
		return "Followed the example"
	case StatusCreatedAFunExample:
		return "Created a fun example"
	default:
		return "Unknown"
	}
}

type GettingStarted struct {
	ReadTheDocs       string
	FollowAnExample   string
	CreateAFunExample string
}

type Example struct {
	Now time.Time
}

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
	Clock         clock.Clock
}

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, Status] {
	b := workflow.NewBuilder[Example, Status]("timeout example")

	b.AddTimeout(StatusStarted,
		func(ctx context.Context, r *workflow.Record[Example, Status], now time.Time) (time.Time, error) {
			// Using "now" over time.Now() allows for you to specify a clock for testing.
			return now.Add(time.Hour), nil
		},
		func(ctx context.Context, r *workflow.Record[Example, Status], now time.Time) (Status, error) {
			r.Object.Now = now
			return StatusFollowedTheExample, nil
		},
		StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
		workflow.WithTimeoutStore(d.TimeoutStore),
	)
}
