package example_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/luno/workflow"
	wexample "github.com/luno/workflow/example"
	"github.com/luno/workflow/example/gettingstarted"

	"github.com/luno/workflow/adapters/kafkastreamer/example"
)

func TestExampleWorkflow(t *testing.T) {
	t.Parallel()

	wf := example.ExampleWorkflow()
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "82347982374982374"
	_, err := wf.Trigger(ctx, foreignID, wexample.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, foreignID, wexample.StatusReadTheDocs, gettingstarted.GettingStarted{
		ReadTheDocs: "✅",
	})

	workflow.Require(t, wf, foreignID, wexample.StatusFollowedTheExample, gettingstarted.GettingStarted{
		ReadTheDocs:     "✅",
		FollowAnExample: "✅",
	})

	workflow.Require(t, wf, foreignID, wexample.StatusCreatedAFunExample, gettingstarted.GettingStarted{
		ReadTheDocs:       "✅",
		FollowAnExample:   "✅",
		CreateAFunExample: "✅",
	})
}
