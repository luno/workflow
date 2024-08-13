package schedule

import (
	"context"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
	"github.com/luno/workflow/example"
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

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, example.Status] {
	b := workflow.NewBuilder[Example, example.Status]("schedule trigger example")

	b.AddStep(example.StatusStarted,
		func(ctx context.Context, r *workflow.Record[Example, example.Status]) (example.Status, error) {
			return example.StatusFollowedTheExample, nil
		},
		example.StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
	)
}
