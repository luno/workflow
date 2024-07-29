package workflow_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestSchedule(t *testing.T) {
	workflowName := "sync users"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Record[MyType, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Record[MyType, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd)

	now := time.Date(2023, time.April, 9, 8, 30, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithDebugMode(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	go func() {
		err := wf.Schedule("andrew", StatusStart, "@monthly")
		jtest.RequireNil(t, err)
	}()

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	_, err := recordStore.Latest(ctx, workflowName, "andrew")
	// Expect there to be no entries yet
	jtest.Require(t, workflow.ErrRecordNotFound, err)

	// Grab the time from the clock for expectation as to the time we expect the entry to have
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	firstScheduled, err := recordStore.Latest(ctx, workflowName, "andrew")
	jtest.RequireNil(t, err)

	_, err = wf.Await(ctx, firstScheduled.ForeignID, firstScheduled.RunID, StatusEnd)
	jtest.RequireNil(t, err)

	expectedTimestamp = time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	secondScheduled, err := recordStore.Latest(ctx, workflowName, "andrew")
	jtest.RequireNil(t, err)

	require.NotEqual(t, firstScheduled.RunID, secondScheduled.RunID)
}

func TestWorkflow_ScheduleShutdown(t *testing.T) {
	b := workflow.NewBuilder[MyType, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Record[MyType, status]) (status, error) {
		return 0, nil
	}, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
		workflow.WithDebugMode(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("andrew", StatusStart, "@monthly")
		jtest.RequireNil(t, err)
	}()

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, map[string]workflow.State{
		"start-andrew-scheduler-@monthly": workflow.StateRunning,
		"start-consumer-1-of-1":           workflow.StateRunning,
		"outbox-consumer-1-of-1":          workflow.StateRunning,
		"example-delete-consumer":         workflow.StateRunning,
	}, wf.States())

	wf.Stop()

	require.Equal(t, map[string]workflow.State{
		"start-andrew-scheduler-@monthly": workflow.StateShutdown,
		"start-consumer-1-of-1":           workflow.StateShutdown,
		"outbox-consumer-1-of-1":          workflow.StateShutdown,
		"example-delete-consumer":         workflow.StateShutdown,
	}, wf.States())
}

func TestWorkflow_ScheduleFilter(t *testing.T) {
	workflowName := "sync users"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Record[MyType, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Record[MyType, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd)

	now := time.Date(2023, time.April, 9, 8, 30, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	recordStore := memrecordstore.New()
	wf := b.Build(
		memstreamer.New(),
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithDebugMode(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	skipVal := false
	shouldSkip := &skipVal
	filter := func(ctx context.Context) (bool, error) {
		return *shouldSkip, nil
	}
	opt := workflow.WithScheduleFilter[MyType, status](filter)

	go func() {
		err := wf.Schedule("andrew", StatusStart, "@monthly", opt)
		jtest.RequireNil(t, err)
	}()

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	_, err := recordStore.Latest(ctx, workflowName, "andrew")
	// Expect there to be no entries yet
	jtest.Require(t, workflow.ErrRecordNotFound, err)

	// Grab the time from the clock for expectation as to the time we expect the entry to have
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	_, err = recordStore.Latest(ctx, workflowName, "andrew")
	// Expect there to be no entries yet
	jtest.Require(t, workflow.ErrRecordNotFound, err)

	// Disable the filter to enable scheduling
	*shouldSkip = true

	expectedTimestamp = time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	latest, err := recordStore.Latest(ctx, workflowName, "andrew")
	jtest.RequireNil(t, err)

	resp, err := wf.Await(ctx, latest.ForeignID, latest.RunID, StatusEnd)
	jtest.RequireNil(t, err)

	require.Equal(t, expectedTimestamp, resp.CreatedAt)
}
