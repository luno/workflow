package gettingstarted

import (
	"context"

	"github.com/luno/workflow"
	"github.com/luno/workflow/example"
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

func Workflow(d Deps) *workflow.Workflow[GettingStarted, example.Status] {
	b := workflow.NewBuilder[GettingStarted, example.Status]("getting started")

	b.AddStep(example.StatusStarted, func(ctx context.Context, r *workflow.Record[GettingStarted, example.Status]) (example.Status, error) {
		r.Object.ReadTheDocs = "✅"
		return example.StatusReadTheDocs, nil
	}, example.StatusReadTheDocs)

	b.AddStep(example.StatusReadTheDocs, func(ctx context.Context, r *workflow.Record[GettingStarted, example.Status]) (example.Status, error) {
		r.Object.FollowAnExample = "✅"
		return example.StatusFollowedTheExample, nil
	}, example.StatusFollowedTheExample)

	b.AddStep(example.StatusFollowedTheExample, func(ctx context.Context, r *workflow.Record[GettingStarted, example.Status]) (example.Status, error) {
		r.Object.CreateAFunExample = "✅"
		return example.StatusCreatedAFunExample, nil
	}, example.StatusCreatedAFunExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
