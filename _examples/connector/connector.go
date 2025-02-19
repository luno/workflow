package connector

import (
	"context"

	"github.com/luno/workflow"
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

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
	Connector     workflow.ConnectorConstructor
}

func Workflow(d Deps) *workflow.Workflow[GettingStarted, Status] {
	builder := workflow.NewBuilder[GettingStarted, Status]("workflow B")
	builder.AddConnector(
		"my-example-connector",
		d.Connector,
		func(ctx context.Context, api workflow.API[GettingStarted, Status], e *workflow.ConnectorEvent) error {
			_, err := api.Trigger(
				ctx,
				e.ForeignID,
				StatusStarted,
				workflow.WithInitialValue[GettingStarted, Status](&GettingStarted{
					ReadTheDocs: "✅",
				}),
			)
			if err != nil {
				return err
			}

			return nil
		},
	)

	builder.AddStep(StatusStarted,
		func(ctx context.Context, r *workflow.Run[GettingStarted, Status]) (Status, error) {
			r.Object.FollowAnExample = "✅"

			return StatusFollowedTheExample, nil
		},
		StatusFollowedTheExample,
	)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
