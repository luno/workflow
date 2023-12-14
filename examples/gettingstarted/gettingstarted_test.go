package gettingstarted_test

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
	"github.com/andrewwormald/workflow/examples/gettingstarted"
)

func TestWorkflow(t *testing.T) {
	wf := gettingstarted.WorkflowWithEnum(gettingstarted.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID, examples.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, foreignID, examples.StatusReadTheDocs, gettingstarted.GettingStarted{
		ReadTheDocs: "✅",
	})

	workflow.Require(t, wf, foreignID, examples.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})

	workflow.Require(t, wf, foreignID, examples.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})
}
