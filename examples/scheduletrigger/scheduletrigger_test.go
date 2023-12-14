package scheduletrigger_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memstreamer"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
	"github.com/andrewwormald/workflow/examples"
	"github.com/andrewwormald/workflow/examples/scheduletrigger"
)

func TestExampleWorkflow(t *testing.T) {
	recordStore := memrecordstore.New()
	now := time.Now().UTC()
	clock := clocktesting.NewFakeClock(now)
	wf := scheduletrigger.ExampleWorkflow(scheduletrigger.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   recordStore,
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
		Clock:         clock,
	})
	t.Cleanup(wf.Stop)

	ctx := context.Background()
	wf.Run(ctx)

	foreignID := "hourly-run"

	go func() {
		err := wf.ScheduleTrigger(foreignID, examples.StatusStarted, "@hourly")
		jtest.RequireNil(t, err)
	}()

	// Give time for go routine to spin up
	time.Sleep(20 * time.Millisecond)

	_, err := recordStore.Latest(ctx, wf.Name, foreignID)
	// Expect there to be no entries yet
	jtest.Require(t, workflow.ErrRecordNotFound, err)

	clock.Step(time.Hour)

	// Allow scheduling to take place
	time.Sleep(20 * time.Millisecond)

	firstScheduled, err := recordStore.Latest(ctx, wf.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, "schedule trigger example", firstScheduled.WorkflowName)
	require.Equal(t, "hourly-run", firstScheduled.ForeignID)

	clock.Step(time.Hour)

	// Allow scheduling to take place
	time.Sleep(20 * time.Millisecond)

	secondScheduled, err := recordStore.Latest(ctx, wf.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, "schedule trigger example", secondScheduled.WorkflowName)
	require.Equal(t, "hourly-run", secondScheduled.ForeignID)

	require.NotEqual(t, firstScheduled.RunID, secondScheduled.RunID)
}
