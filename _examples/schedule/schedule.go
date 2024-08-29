package schedule

import (
	"context"

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
	EmailConfirmed bool
}

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
	Clock         clock.Clock
}

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, Status] {
	b := workflow.NewBuilder[Example, Status]("schedule trigger example")

	b.AddStep(StatusStarted,
		func(ctx context.Context, r *workflow.Run[Example, Status]) (Status, error) {
			return StatusFollowedTheExample, nil
		},
		StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
	)
}
