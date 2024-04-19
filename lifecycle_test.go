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
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle,
		func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
			return StatusEnd, nil
		},
		StatusEnd,
	).WithOptions(
		workflow.ParallelCount(3),
	)

	b.AddTimeout(StatusInitiated, workflow.DurationTimerFunc[string, status](time.Hour), func(ctx context.Context, r *workflow.Record[string, status], now time.Time) (status, error) {
		return StatusCompleted, nil
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

	require.Equal(t, map[string]workflow.LifecycleState{}, wf.Lifecycles())

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Second)

	require.Equal(t, map[string]workflow.LifecycleState{
		"middle-consumer-1-of-3":                                    workflow.LifecycleStateRunning,
		"middle-consumer-2-of-3":                                    workflow.LifecycleStateRunning,
		"middle-consumer-3-of-3":                                    workflow.LifecycleStateRunning,
		"start-consumer-1-of-1":                                     workflow.LifecycleStateRunning,
		"initiated-timeout-auto-inserter-consumer":                  workflow.LifecycleStateRunning,
		"initiated-timeout-consumer":                                workflow.LifecycleStateRunning,
		"consume-other-stream-connector-to-example-consumer-1-of-2": workflow.LifecycleStateRunning,
		"consume-other-stream-connector-to-example-consumer-2-of-2": workflow.LifecycleStateRunning,
		"outbox-consumer-1-of-1":                                    workflow.LifecycleStateRunning,
	}, wf.Lifecycles())

	wf.Stop()
	require.Equal(t, map[string]workflow.LifecycleState{
		"middle-consumer-1-of-3":                                    workflow.LifecycleStateShutdown,
		"middle-consumer-2-of-3":                                    workflow.LifecycleStateShutdown,
		"middle-consumer-3-of-3":                                    workflow.LifecycleStateShutdown,
		"start-consumer-1-of-1":                                     workflow.LifecycleStateShutdown,
		"initiated-timeout-auto-inserter-consumer":                  workflow.LifecycleStateShutdown,
		"initiated-timeout-consumer":                                workflow.LifecycleStateShutdown,
		"consume-other-stream-connector-to-example-consumer-1-of-2": workflow.LifecycleStateShutdown,
		"consume-other-stream-connector-to-example-consumer-2-of-2": workflow.LifecycleStateShutdown,
		"outbox-consumer-1-of-1":                                    workflow.LifecycleStateShutdown,
	}, wf.Lifecycles())
}
