package connectors

import (
	"context"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
	"github.com/luno/workflow/examples/gettingstarted"
)

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
	Connector     workflow.ConnectorConstructor
}

func Workflow(d Deps) *workflow.Workflow[gettingstarted.GettingStarted, examples.Status] {
	builder := workflow.NewBuilder[gettingstarted.GettingStarted, examples.Status]("workflow B")
	builder.AddConnector(
		"my-example-connector",
		d.Connector,
		func(ctx context.Context, w *workflow.Workflow[gettingstarted.GettingStarted, examples.Status], e *workflow.ConnectorEvent) error {
			_, err := w.Trigger(ctx, e.ForeignID, examples.StatusStarted, workflow.WithInitialValue[gettingstarted.GettingStarted, examples.Status](&gettingstarted.GettingStarted{
				ReadTheDocs: "✅",
			}))
			if err != nil {
				return err
			}

			return nil
		},
	)

	builder.AddStep(examples.StatusStarted,
		func(ctx context.Context, r *workflow.Record[gettingstarted.GettingStarted, examples.Status]) (examples.Status, error) {
			r.Object.FollowAnExample = "✅"

			return examples.StatusFollowedTheExample, nil
		},
		examples.StatusFollowedTheExample,
	)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
