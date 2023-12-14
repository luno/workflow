package connectors

import (
	"context"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/examples"
)

type WorkflowADeps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
}

type TypeA struct {
	Value string
}

func WorkflowA(d WorkflowADeps) *workflow.Workflow[TypeA, examples.Status] {
	builder := workflow.NewBuilder[TypeA, examples.Status]("workflow A")

	builder.AddStep(examples.StatusStarted, func(ctx context.Context, r *workflow.Record[TypeA, examples.Status]) (bool, error) {
		r.Object.Value = "Hello"
		return true, nil
	}, examples.StatusFollowedTheExample)

	builder.AddStep(examples.StatusFollowedTheExample, func(ctx context.Context, r *workflow.Record[TypeA, examples.Status]) (bool, error) {
		r.Object.Value += " World"
		return true, nil
	}, examples.StatusCreatedAFunExample)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
	)
}

type WorkflowBDeps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler

	WorkflowAStreamer    workflow.EventStreamer
	WorkflowARecordStore workflow.RecordStore
}

type TypeB struct {
	Value string
}

func WorkflowB(d WorkflowBDeps) *workflow.Workflow[TypeB, examples.Status] {
	builder := workflow.NewBuilder[TypeB, examples.Status]("workflow B")

	builder.AddStep(examples.StatusStarted, func(ctx context.Context, r *workflow.Record[TypeB, examples.Status]) (bool, error) {
		return true, nil
	}, examples.StatusFollowedTheExample)

	builder.AddWorkflowConnector(
		workflow.WorkflowConnectionDetails{
			WorkflowName: "workflow A",
			Status:       int(examples.StatusCreatedAFunExample),
			Stream:       d.WorkflowAStreamer,
		},
		func(ctx context.Context, e *workflow.Event) (foreignID string, err error) {
			return e.Headers[workflow.HeaderWorkflowForeignID], nil
		},
		examples.StatusFollowedTheExample,
		func(ctx context.Context, r *workflow.Record[TypeB, examples.Status], e *workflow.Event) (bool, error) {
			recordA, err := d.WorkflowARecordStore.Lookup(ctx, e.ForeignID)
			if err != nil {
				return false, err
			}

			var objectA TypeB
			err = workflow.Unmarshal(recordA.Object, &objectA)
			if err != nil {
				return false, err
			}

			r.Object.Value = objectA.Value

			return true, nil
		},
		examples.StatusCreatedAFunExample,
	)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
	)
}
