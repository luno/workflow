package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestAwait(t *testing.T) {
	b := workflow.NewBuilder[string, status]("consumer lag")
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			*r.Object = "hello world"
			return StatusEnd, nil
		},
		StatusEnd,
	)
	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := t.Context()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	runID, err := wf.Trigger(ctx, "1")
	require.NoError(t, err)

	res, err := wf.Await(ctx, "1", runID, StatusEnd, workflow.WithAwaitPollingFrequency(10*time.Nanosecond))
	require.NoError(t, err)

	require.Equal(t, StatusEnd, res.Status)
	require.Equal(t, "hello world", *res.Object)
}

func TestWaitForComplete(t *testing.T) {
	b := workflow.NewBuilder[string, status]("wait for complete")
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			*r.Object = "completed workflow"
			return StatusEnd, nil
		},
		StatusEnd,
	)
	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := t.Context()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	runID, err := wf.Trigger(ctx, "1")
	require.NoError(t, err)

	res, err := wf.WaitForComplete(ctx, "1", runID, workflow.WithAwaitPollingFrequency(10*time.Nanosecond))
	require.NoError(t, err)

	require.Equal(t, StatusEnd, res.Status)
	require.Equal(t, "completed workflow", *res.Object)
	require.Equal(t, workflow.RunStateCompleted, res.RunState)
}

func TestWaitForCompleteWithMultipleTerminalStatuses(t *testing.T) {
	b := workflow.NewBuilder[string, status]("wait for complete with branches")
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			if *r.Object == "success" {
				return StatusEnd, nil
			}
			return StatusMiddle, nil
		},
		StatusEnd,
		StatusMiddle,
	)
	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := t.Context()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	t.Run("completes with StatusEnd", func(t *testing.T) {
		value := "success"
		runID, err := wf.Trigger(ctx, "1", workflow.WithInitialValue[string, status](&value))
		require.NoError(t, err)

		res, err := wf.WaitForComplete(ctx, "1", runID, workflow.WithAwaitPollingFrequency(10*time.Nanosecond))
		require.NoError(t, err)

		require.Equal(t, StatusEnd, res.Status)
		require.Equal(t, "success", *res.Object)
		require.Equal(t, workflow.RunStateCompleted, res.RunState)
	})

	t.Run("completes with StatusMiddle", func(t *testing.T) {
		value := "alternate"
		runID, err := wf.Trigger(ctx, "2", workflow.WithInitialValue[string, status](&value))
		require.NoError(t, err)

		res, err := wf.WaitForComplete(ctx, "2", runID, workflow.WithAwaitPollingFrequency(10*time.Nanosecond))
		require.NoError(t, err)

		require.Equal(t, StatusMiddle, res.Status)
		require.Equal(t, "alternate", *res.Object)
		require.Equal(t, workflow.RunStateCompleted, res.RunState)
	})
}
