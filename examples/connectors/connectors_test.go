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

	workflowB := connectors.WorkflowB(connectors.WorkflowBDeps{
		EventStreamer:        memstreamer.New(),
		RecordStore:          memrecordstore.New(),
		TimeoutStore:         memtimeoutstore.New(),
		RoleScheduler:        memrolescheduler.New(),
		WorkflowAStreamer:    eventStreamerA,
		WorkflowARecordStore: recordStoreA,
	})

	workflowB.Run(ctx)
	t.Cleanup(workflowB.Stop)

	foreignID := "andrewwormald"

	runID, err := workflowB.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	_, err = workflowB.Await(ctx, foreignID, runID, examples.StatusFollowedTheExample)
	jtest.RequireNil(t, err)

	_, err = workflowA.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	// Wait until workflowB reaches "Finished waiting" before finishing the test. Note that the value is "Hello World"
	// from workflow A which is because we copy that value from workflow A to workflow B in the example implementation:
	// examples/connectors/connectors.go:80
	workflow.Require(t, workflowB, foreignID, examples.StatusCreatedAFunExample, connectors.TypeB{
		Value: "Hello World",
	})
}
