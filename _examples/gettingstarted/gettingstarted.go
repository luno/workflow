package gettingstarted

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
}

func Workflow(d Deps) *workflow.Workflow[GettingStarted, Status] {
	b := workflow.NewBuilder[GettingStarted, Status]("getting started")

	b.AddStep(StatusStarted, func(ctx context.Context, r *workflow.Record[GettingStarted, Status]) (Status, error) {
		r.Object.ReadTheDocs = "✅"
		return StatusReadTheDocs, nil
	}, StatusReadTheDocs)

	b.AddStep(StatusReadTheDocs, func(ctx context.Context, r *workflow.Record[GettingStarted, Status]) (Status, error) {
		r.Object.FollowAnExample = "✅"
		return StatusFollowedTheExample, nil
	}, StatusFollowedTheExample)

	b.AddStep(StatusFollowedTheExample, func(ctx context.Context, r *workflow.Record[GettingStarted, Status]) (Status, error) {
		r.Object.CreateAFunExample = "✅"
		return StatusCreatedAFunExample, nil
	}, StatusCreatedAFunExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
