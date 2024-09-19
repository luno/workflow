package schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/_examples/schedule"
)

func TestExampleWorkflow(t *testing.T) {
	recordStore := memrecordstore.New()
	now := time.Now().UTC()
	clock := clocktesting.NewFakeClock(now)
	wf := schedule.ExampleWorkflow(schedule.Deps{
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
		err := wf.Schedule(foreignID, schedule.StatusStarted, "@hourly")
		require.Nil(t, err)
	}()

	// Give time for go routine to spin up
	time.Sleep(200 * time.Millisecond)

	_, err := recordStore.Latest(ctx, wf.Name, foreignID)
	// Expect there to be no entries yet
	require.True(t, errors.Is(err, workflow.ErrRecordNotFound))

	clock.Step(time.Hour)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	firstScheduled, err := recordStore.Latest(ctx, wf.Name, foreignID)
	require.Nil(t, err)

	require.Equal(t, "schedule trigger example", firstScheduled.WorkflowName)
	require.Equal(t, "hourly-run", firstScheduled.ForeignID)

	clock.Step(time.Hour)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	secondScheduled, err := recordStore.Latest(ctx, wf.Name, foreignID)
	require.Nil(t, err)

	require.Equal(t, "schedule trigger example", secondScheduled.WorkflowName)
	require.Equal(t, "hourly-run", secondScheduled.ForeignID)

	require.NotEqual(t, firstScheduled.RunID, secondScheduled.RunID)
}
