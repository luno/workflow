package connectors

import (
	"context"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
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

	WorkflowARecordStore    workflow.RecordStore
	WorkflowAConsumerStream workflow.Consumer
}

type TypeB struct {
	Value string
}

func WorkflowB(d WorkflowBDeps) *workflow.Workflow[TypeB, examples.Status] {
	builder := workflow.NewBuilder[TypeB, examples.Status]("workflow B")

	builder.AddConnector(
		"my-example-connector",
		d.WorkflowAConsumerStream,
		func(ctx context.Context, w *workflow.Workflow[TypeB, examples.Status], e *workflow.Event) error {
			recordA, err := d.WorkflowARecordStore.Lookup(ctx, e.ForeignID)
			if err != nil {
				return err
			}

			var objectA TypeB
			err = workflow.Unmarshal(recordA.Object, &objectA)
			if err != nil {
				return err
			}

			_, err = w.Trigger(ctx, recordA.ForeignID, examples.StatusStarted, workflow.WithInitialValue[TypeB, examples.Status](&TypeB{
				Value: objectA.Value,
			}))
			if err != nil {
				return err
			}

			return nil
		},
	)

	builder.AddStep(examples.StatusStarted, func(ctx context.Context, r *workflow.Record[TypeB, examples.Status]) (bool, error) {
		r.Object.Value += ", I have been made from two workflows"

		return true, nil
	}, examples.StatusFollowedTheExample)

	return builder.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
	)
}
