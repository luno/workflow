package kafka_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
	"github.com/luno/workflow/examples/gettingstarted"
	"github.com/luno/workflow/examples/kafka"
)

func TestExampleWorkflow(t *testing.T) {
	t.Parallel()

	wf := kafka.ExampleWorkflow()
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
