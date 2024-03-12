package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
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

	ctx := context.Background()
	streamConsumer, err := memstreamer.New().NewConsumer(ctx, "", "")
	jtest.RequireNil(t, err)

	b.AddConnector(
		"consume-other-stream",
		streamConsumer,
		func(ctx context.Context, w *workflow.Workflow[string, status], e *workflow.Event) error {
			return nil
		},
		workflow.WithConnectorParallelCount(2),
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

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Second)

	require.Equal(t, map[string]workflow.State{
		"middle-to-end-consumer-1-of-3":                             workflow.StateRunning,
		"middle-to-end-consumer-2-of-3":                             workflow.StateRunning,
		"middle-to-end-consumer-3-of-3":                             workflow.StateRunning,
		"start-to-middle-consumer-1-of-1":                           workflow.StateRunning,
		"initiated-timeout-auto-inserter-consumer":                  workflow.StateRunning,
		"initiated-timeout-consumer":                                workflow.StateRunning,
		"consume-other-stream-connector-to-example-consumer-1-of-2": workflow.StateRunning,
		"consume-other-stream-connector-to-example-consumer-2-of-2": workflow.StateRunning,
		"outbox-consumer-1-of-1":                                    workflow.StateRunning,
	}, wf.States())

	wf.Stop()
	require.Equal(t, map[string]workflow.State{
		"middle-to-end-consumer-1-of-3":                             workflow.StateShutdown,
		"middle-to-end-consumer-2-of-3":                             workflow.StateShutdown,
		"middle-to-end-consumer-3-of-3":                             workflow.StateShutdown,
		"start-to-middle-consumer-1-of-1":                           workflow.StateShutdown,
		"initiated-timeout-auto-inserter-consumer":                  workflow.StateShutdown,
		"initiated-timeout-consumer":                                workflow.StateShutdown,
		"consume-other-stream-connector-to-example-consumer-1-of-2": workflow.StateShutdown,
		"consume-other-stream-connector-to-example-consumer-2-of-2": workflow.StateShutdown,
		"outbox-consumer-1-of-1":                                    workflow.StateShutdown,
	}, wf.States())
}
