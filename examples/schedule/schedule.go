package schedule

import (
	"context"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
)

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

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, examples.Status] {
	b := workflow.NewBuilder[Example, examples.Status]("schedule trigger example")

	b.AddStep(examples.StatusStarted,
		func(ctx context.Context, r *workflow.Record[Example, examples.Status]) (examples.Status, error) {
			return examples.StatusFollowedTheExample, nil
		},
		examples.StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
	)
}
