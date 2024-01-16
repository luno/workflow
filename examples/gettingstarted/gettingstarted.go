package gettingstarted

import (
	"context"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
)

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
}

func WorkflowWithEnum(d Deps) *workflow.Workflow[GettingStarted, examples.Status] {
	b := workflow.NewBuilder[GettingStarted, examples.Status]("getting started")

	b.AddStep(examples.StatusStarted, func(ctx context.Context, r *workflow.Record[GettingStarted, examples.Status]) (bool, error) {
		r.Object.ReadTheDocs = "✅"
		return true, nil
	}, examples.StatusReadTheDocs)

	b.AddStep(examples.StatusReadTheDocs, func(ctx context.Context, r *workflow.Record[GettingStarted, examples.Status]) (bool, error) {
		r.Object.FollowAnExample = "✅"
		return true, nil
	}, examples.StatusFollowedTheExample)

	b.AddStep(examples.StatusFollowedTheExample, func(ctx context.Context, r *workflow.Record[GettingStarted, examples.Status]) (bool, error) {
		r.Object.CreateAFunExample = "✅"
		return true, nil
	}, examples.StatusCreatedAFunExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
	)
}
