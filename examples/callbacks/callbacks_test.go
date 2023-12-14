package callbacks_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memstreamer"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
	"github.com/andrewwormald/workflow/examples"
	"github.com/andrewwormald/workflow/examples/callbacks"
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
	runID, err := wf.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.TriggerCallbackOn(t, wf, foreignID, runID, examples.StatusStarted, callbacks.EmailConfirmationResponse{
		Confirmed: true,
	})

	workflow.Require(t, wf, foreignID, examples.StatusFollowedTheExample, callbacks.Example{
		EmailConfirmed: true,
	})
}
