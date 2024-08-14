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

func Workflow(d Deps) *workflow.Workflow[GettingStarted, examples.Status] {
	b := workflow.NewBuilder[GettingStarted, examples.Status]("getting started")

	b.AddStep(examples.StatusStarted, func(ctx context.Context, r *workflow.Run[GettingStarted, examples.Status]) (examples.Status, error) {
		r.Object.ReadTheDocs = "✅"
		return examples.StatusReadTheDocs, nil
	}, examples.StatusReadTheDocs)

	b.AddStep(examples.StatusReadTheDocs, func(ctx context.Context, r *workflow.Run[GettingStarted, examples.Status]) (examples.Status, error) {
		r.Object.FollowAnExample = "✅"
		return examples.StatusFollowedTheExample, nil
	}, examples.StatusFollowedTheExample)

	b.AddStep(examples.StatusFollowedTheExample, func(ctx context.Context, r *workflow.Run[GettingStarted, examples.Status]) (examples.Status, error) {
		r.Object.CreateAFunExample = "✅"
		return examples.StatusCreatedAFunExample, nil
	}, examples.StatusCreatedAFunExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
