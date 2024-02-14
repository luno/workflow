package connectors_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/examples"
	"github.com/luno/workflow/examples/connectors"
)

func TestConnectStreamParallelConsumer(t *testing.T) {
	ctx := context.Background()
	eventStreamerA := memstreamer.New()
	recordStoreA := memrecordstore.New()

	workflowA := connectors.WorkflowA(connectors.WorkflowADeps{
		EventStreamer: eventStreamerA,
		RecordStore:   recordStoreA,
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})

	workflowA.Run(ctx)
	t.Cleanup(workflowA.Stop)

	topic := workflow.Topic("workflow A", int(examples.StatusCreatedAFunExample))
	consumerStream, err := eventStreamerA.NewConsumer(topic, workflowA.Name)
	jtest.RequireNil(t, err)

	workflowB := connectors.WorkflowB(connectors.WorkflowBDeps{
		EventStreamer:           memstreamer.New(),
		RecordStore:             memrecordstore.New(),
		TimeoutStore:            memtimeoutstore.New(),
		RoleScheduler:           memrolescheduler.New(),
		WorkflowARecordStore:    recordStoreA,
		WorkflowAConsumerStream: consumerStream,
	})

	workflowB.Run(ctx)
	t.Cleanup(workflowB.Stop)

	foreignID := "andrewwormald"

	_, err = workflowA.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, workflowB, foreignID, examples.StatusFollowedTheExample, connectors.TypeB{
		Value: "Hello World, I have been made from two workflows",
	})
}
