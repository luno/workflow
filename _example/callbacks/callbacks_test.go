package callbacks_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/example"
	"github.com/luno/workflow/example/callbacks"
)

func TestCallbackWorkflow(t *testing.T) {
	wf := callbacks.ExampleWorkflow(callbacks.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "andrew"
	runID, err := wf.Trigger(ctx, foreignID, example.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.TriggerCallbackOn(t, wf, foreignID, runID, example.StatusStarted, callbacks.EmailConfirmationResponse{
		Confirmed: true,
	})

	workflow.Require(t, wf, foreignID, example.StatusFollowedTheExample, callbacks.Example{
		EmailConfirmed: true,
	})
}
