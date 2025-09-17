package timeout_test

import (
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/_examples/timeout"
)

func TestTimeoutWorkflow(t *testing.T) {
	now := time.Now().UTC()
	clock := clocktesting.NewFakeClock(now)
	wf := timeout.ExampleWorkflow(timeout.Deps{
		EventStreamer: memstreamer.New(),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
		Clock:         clock,
	})
	t.Cleanup(wf.Stop)

	ctx := t.Context()
	wf.Run(ctx)

	foreignID := "andrew"
	runID, err := wf.Trigger(ctx, foreignID)
	require.NoError(t, err)
	require.NotEmpty(t, runID)

	workflow.AwaitTimeoutInsert(t, wf, foreignID, runID, timeout.StatusStarted)

	clock.Step(time.Hour)

	workflow.Require(t, wf, foreignID, timeout.StatusFollowedTheExample, timeout.Example{
		Now: clock.Now(),
	})
}
