package workflow_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestRetryOfPausedRecords(t *testing.T) {
	b := workflow.NewBuilder[string, status]("example")
	var errorCounter int
	pauseAfterErrCount := 3
	stopErroringAfter := 5
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			if errorCounter == stopErroringAfter {
				return StatusEnd, nil
			}

			errorCounter++
			return 0, fmt.Errorf("test error")
		},
		StatusEnd,
	).WithOptions(
		workflow.PauseAfterErrCount(pauseAfterErrCount),
		workflow.ErrBackOff(time.Millisecond),
	)
	w := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
		workflow.WithPauseRetry(time.Millisecond*10),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	fid := "12345"
	_, err := w.Trigger(ctx, fid, StatusStart)
	require.NoError(t, err)

	workflow.Require(t, w, fid, StatusEnd, "")
	require.Equal(t, 5, errorCounter)
}

func TestRetryOfPausedRecordsConfig(t *testing.T) {
	newBuilder := func() *workflow.Builder[string, status] {
		b := workflow.NewBuilder[string, status]("example")
		b.AddStep(
			StatusStart,
			func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
				return 0, fmt.Errorf("test error")
			},
			StatusEnd,
		).WithOptions()
		return b
	}

	t.Run("Disabled - no retry process for paused records", func(t *testing.T) {
		w := newBuilder().Build(
			memstreamer.New(),
			memrecordstore.New(),
			memrolescheduler.New(),
			workflow.DisablePauseRetry(),
		)

		ctx := context.Background()
		w.Run(ctx)
		t.Cleanup(w.Stop)

		states := w.States()
		_, processLaunched := states["paused-records-retry-consumer"]
		require.False(t, processLaunched)
	})

	t.Run("Enabled by default should be launched", func(t *testing.T) {
		w := newBuilder().Build(
			memstreamer.New(),
			memrecordstore.New(),
			memrolescheduler.New(),
		)

		ctx := context.Background()
		w.Run(ctx)
		t.Cleanup(w.Stop)

		states := w.States()
		_, processLaunched := states["paused-records-retry-consumer"]
		require.True(t, processLaunched)
	})
}
