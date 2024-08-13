package timeouts

import (
	"context"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
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

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, examples.Status] {
	b := workflow.NewBuilder[Example, examples.Status]("timeout example")

	b.AddTimeout(examples.StatusStarted,
		func(ctx context.Context, r *workflow.Record[Example, examples.Status], now time.Time) (time.Time, error) {
			// Using "now" over time.Now() allows for you to specify a clock for testing.
			return now.Add(time.Hour), nil
		},
		func(ctx context.Context, r *workflow.Record[Example, examples.Status], now time.Time) (examples.Status, error) {
			r.Object.Now = now
			return examples.StatusFollowedTheExample, nil
		},
		examples.StatusFollowedTheExample,
	)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
		workflow.WithClock(d.Clock),
	)
}
