package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memstreamer"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
)

func TestInternalState(t *testing.T) {
	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusEnd, workflow.WithParallelCount(3))

	b.AddTimeout(StatusInitiated, workflow.DurationTimerFunc[string, status](time.Hour), func(ctx context.Context, r *workflow.Record[string, status], now time.Time) (bool, error) {
		return true, nil
	}, StatusCompleted)

	b.AddWorkflowConnector(
		workflow.WorkflowConnectionDetails{
			WorkflowName: "other workflow",
			Status:       int(StatusCompleted),
			Stream:       memstreamer.New(),
		},
		func(ctx context.Context, e *workflow.Event) (string, error) {
			return e.Headers[workflow.HeaderWorkflowForeignID], nil
		},
		StatusMiddle,
		func(ctx context.Context, r *workflow.Record[string, status], e *workflow.Event) (bool, error) {
			return true, nil
		},
		StatusEnd,
		workflow.WithParallelCount(2),
	)

	recordStore := memrecordstore.New()
	timeoutStore := memtimeoutstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		timeoutStore,
		memrolescheduler.New(),
	)

	require.Equal(t, map[string]workflow.State{}, wf.States())

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Second)

	require.Equal(t, map[string]workflow.State{
		"middle-to-end-consumer-1-of-3":                                               workflow.StateRunning,
		"middle-to-end-consumer-2-of-3":                                               workflow.StateRunning,
		"middle-to-end-consumer-3-of-3":                                               workflow.StateRunning,
		"start-to-middle-consumer-1-of-1":                                             workflow.StateRunning,
		"initiated-timeout-auto-inserter-consumer":                                    workflow.StateRunning,
		"initiated-timeout-consumer":                                                  workflow.StateRunning,
		"other_workflow-8-connection-example-middle-to-end-connector-consumer-1-of-2": workflow.StateRunning,
		"other_workflow-8-connection-example-middle-to-end-connector-consumer-2-of-2": workflow.StateRunning,
	}, wf.States())

	wf.Stop()
	require.Equal(t, map[string]workflow.State{
		"middle-to-end-consumer-1-of-3":                                               workflow.StateShutdown,
		"middle-to-end-consumer-2-of-3":                                               workflow.StateShutdown,
		"middle-to-end-consumer-3-of-3":                                               workflow.StateShutdown,
		"start-to-middle-consumer-1-of-1":                                             workflow.StateShutdown,
		"initiated-timeout-auto-inserter-consumer":                                    workflow.StateShutdown,
		"initiated-timeout-consumer":                                                  workflow.StateShutdown,
		"other_workflow-8-connection-example-middle-to-end-connector-consumer-1-of-2": workflow.StateShutdown,
		"other_workflow-8-connection-example-middle-to-end-connector-consumer-2-of-2": workflow.StateShutdown,
	}, wf.States())
}
