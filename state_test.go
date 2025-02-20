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
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

func TestInternalState(t *testing.T) {
	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return StatusEnd, nil
		},
		StatusEnd,
	).WithOptions(
		workflow.ParallelCount(3),
	)

	b.AddTimeout(StatusInitiated, workflow.DurationTimerFunc[string, status](time.Hour), func(ctx context.Context, r *workflow.Run[string, status], now time.Time) (status, error) {
		return StatusCompleted, nil
	}, StatusCompleted)

	ctx := context.Background()

	b.AddConnector(
		"consume-other-stream",
		memstreamer.NewConnector(nil),
		func(ctx context.Context, api workflow.API[string, status], e *workflow.ConnectorEvent) error {
			return nil
		},
	).WithOptions(
		workflow.ParallelCount(2),
	)

	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
	)

	require.Equal(t, map[string]workflow.State{}, wf.States())

	wf.Run(ctx)
	wf.Stop()

	require.Equal(t, map[string]workflow.State{
		"middle-consumer-1-of-3":                                    workflow.StateShutdown,
		"middle-consumer-2-of-3":                                    workflow.StateShutdown,
		"middle-consumer-3-of-3":                                    workflow.StateShutdown,
		"start-consumer-1-of-1":                                     workflow.StateShutdown,
		"initiated-timeout-auto-inserter-consumer":                  workflow.StateShutdown,
		"initiated-timeout-consumer":                                workflow.StateShutdown,
		"consume-other-stream-connector-to-example-consumer-1-of-2": workflow.StateShutdown,
		"consume-other-stream-connector-to-example-consumer-2-of-2": workflow.StateShutdown,
		"outbox-consumer-1-of-1":                                    workflow.StateShutdown,
		"example-delete-consumer":                                   workflow.StateShutdown,
		"example-paused-records-auto-retry":                         workflow.StateShutdown,
	}, wf.States())
}
