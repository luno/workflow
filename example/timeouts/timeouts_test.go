package timeouts_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/example"
	"github.com/luno/workflow/example/timeouts"
)

func TestTimeoutWorkflow(t *testing.T) {
	now := time.Now().UTC()
	clock := clocktesting.NewFakeClock(now)
	wf := timeouts.ExampleWorkflow(timeouts.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
		Clock:         clock,
	})
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "andrew"
	runID, err := wf.Trigger(ctx, foreignID, example.StatusStarted)
	jtest.RequireNil(t, err)

	workflow.AwaitTimeoutInsert(t, wf, foreignID, runID, example.StatusStarted)

	clock.Step(time.Hour)

	workflow.Require(t, wf, foreignID, example.StatusFollowedTheExample, timeouts.Example{
		Now: clock.Now(),
	})
}
