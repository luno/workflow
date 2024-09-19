package callback_test

import (
	"context"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/_examples/callback"
)

func TestCallbackWorkflow(t *testing.T) {
	wf := callback.ExampleWorkflow(callback.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "andrew"
	runID, err := wf.Trigger(ctx, foreignID, callback.StatusStarted)
	require.Nil(t, err)

	workflow.TriggerCallbackOn(t, wf, foreignID, runID, callback.StatusStarted, callback.EmailConfirmationResponse{
		Confirmed: true,
	})

	workflow.Require(t, wf, foreignID, callback.StatusFollowedTheExample, callback.Example{
		EmailConfirmed: true,
	})
}
