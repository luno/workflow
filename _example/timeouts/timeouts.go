package timeouts

import (
	"context"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
	"github.com/luno/workflow/example"
)

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

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, example.Status] {
	b := workflow.NewBuilder[Example, example.Status]("timeout example")

	b.AddTimeout(example.StatusStarted,
		func(ctx context.Context, r *workflow.Record[Example, example.Status], now time.Time) (time.Time, error) {
			// Using "now" over time.Now() allows for you to specify a clock for testing.
			return now.Add(time.Hour), nil
		},
		func(ctx context.Context, r *workflow.Record[Example, example.Status], now time.Time) (example.Status, error) {
			r.Object.Now = now
			return example.StatusFollowedTheExample, nil
		},
		example.StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
		workflow.WithTimeoutStore(d.TimeoutStore),
	)
}
