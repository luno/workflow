package connectors

import (
	"context"

	"github.com/luno/workflow"
	"github.com/luno/workflow/example"
	"github.com/luno/workflow/example/gettingstarted"
)

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
	Connector     workflow.ConnectorConstructor
}

func Workflow(d Deps) *workflow.Workflow[gettingstarted.GettingStarted, example.Status] {
	builder := workflow.NewBuilder[gettingstarted.GettingStarted, example.Status]("workflow B")
	builder.AddConnector(
		"my-example-connector",
		d.Connector,
		func(ctx context.Context, w *workflow.Workflow[gettingstarted.GettingStarted, example.Status], e *workflow.ConnectorEvent) error {
			_, err := w.Trigger(ctx, e.ForeignID, example.StatusStarted, workflow.WithInitialValue[gettingstarted.GettingStarted, example.Status](&gettingstarted.GettingStarted{
				ReadTheDocs: "✅",
			}))
			if err != nil {
				return err
			}

			return nil
		},
	)

	builder.AddStep(example.StatusStarted,
		func(ctx context.Context, r *workflow.Record[gettingstarted.GettingStarted, example.Status]) (example.Status, error) {
			r.Object.FollowAnExample = "✅"

			return example.StatusFollowedTheExample, nil
		},
		example.StatusFollowedTheExample,
	)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
