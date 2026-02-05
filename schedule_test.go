package workflow_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
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
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(time.Millisecond),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("andrew", "@monthly")
		require.NoError(t, err)
	}()
	wg.Wait()

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	_, err := recordStore.Latest(ctx, workflowName, "andrew")
	// Expect there to be no entries yet
	require.True(t, errors.Is(err, workflow.ErrRecordNotFound))

	// Grab the time from the clock for expectation as to the time we expect the entry to have
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	firstScheduled, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.NoError(t, err)

	_, err = wf.Await(ctx, firstScheduled.ForeignID, firstScheduled.RunID, StatusEnd)
	require.NoError(t, err)

	expectedTimestamp = time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	secondScheduled, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.NoError(t, err)

	require.NotEqual(t, firstScheduled.RunID, secondScheduled.RunID)
}

func TestWorkflow_ScheduleShutdown(t *testing.T) {
	b := workflow.NewBuilder[MyType, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
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
		err := wf.Schedule("andrew", "@monthly")
		require.NoError(t, err)
	}()

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, map[string]workflow.State{
		"andrew-scheduler-@monthly":     workflow.StateRunning,
		"start-consumer-1-of-1":         workflow.StateRunning,
		"outbox-consumer":               workflow.StateRunning,
		"delete-consumer":               workflow.StateRunning,
		"paused-records-retry-consumer": workflow.StateRunning,
	}, wf.States())

	wf.Stop()

	require.Equal(t, map[string]workflow.State{
		"andrew-scheduler-@monthly":     workflow.StateShutdown,
		"start-consumer-1-of-1":         workflow.StateShutdown,
		"outbox-consumer":               workflow.StateShutdown,
		"delete-consumer":               workflow.StateShutdown,
		"paused-records-retry-consumer": workflow.StateShutdown,
	}, wf.States())
}

func TestWorkflow_ScheduleFilter(t *testing.T) {
	workflowName := "sync users"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)

	b.AddStep(StatusMiddle, func(ctx context.Context, t *workflow.Run[MyType, status]) (status, error) {
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
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(time.Millisecond),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	shouldSkip := false
	filter := func(ctx context.Context) (bool, error) {
		return !shouldSkip, nil
	}
	opt := workflow.WithScheduleFilter[MyType, status](filter)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("andrew", "@monthly", opt)
		require.NoError(t, err)
	}()
	wg.Wait()

	// Allow scheduling to initialize
	time.Sleep(10 * time.Millisecond)

	// Verify no record exists initially
	_, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.True(t, errors.Is(err, workflow.ErrRecordNotFound))

	// Test 1: Filter allows scheduling (shouldSkip = false)
	// Move to May 1st - first scheduled time
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	firstRun, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.NoError(t, err)

	// Wait for first run to complete
	_, err = wf.Await(ctx, firstRun.ForeignID, firstRun.RunID, StatusEnd)
	require.NoError(t, err)

	// Test 2: Filter blocks scheduling (shouldSkip = true)
	shouldSkip = true

	// Move to June 1st - second scheduled time, but filter should block it
	expectedTimestamp = time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling attempt to take place
	time.Sleep(10 * time.Millisecond)

	// Should still be the same run as before since scheduling was blocked
	latest, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.NoError(t, err)
	require.Equal(t, firstRun.RunID, latest.RunID, "No new run should be created when filter returns false")

	// Test 3: Filter allows scheduling again (shouldSkip = false)
	shouldSkip = false

	// Move to July 1st - third scheduled time, filter should allow it
	expectedTimestamp = time.Date(2023, time.July, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	secondRun, err := recordStore.Latest(ctx, workflowName, "andrew")
	require.NoError(t, err)
	require.NotEqual(t, firstRun.RunID, secondRun.RunID, "New run should be created when filter returns true")
}

func TestWorkflow_ScheduleWithInitialValue(t *testing.T) {
	workflowName := "initial value test"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, run *workflow.Run[MyType, status]) (status, error) {
		require.Equal(t, "test-email@example.com", run.Object.Email)
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
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(time.Millisecond),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	initialValue := &MyType{
		UserID: 123,
		Email:  "test-email@example.com",
	}
	opt := workflow.WithScheduleInitialValue[MyType, status](initialValue)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("test", "@monthly", opt)
		require.NoError(t, err)
	}()
	wg.Wait()

	// Allow scheduler goroutine to register before advancing clock
	time.Sleep(200 * time.Millisecond)

	// Move to May 1st - first scheduled time
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	run, err := recordStore.Latest(ctx, workflowName, "test")
	require.NoError(t, err)

	// Wait for run to complete and verify initial value was used
	_, err = wf.Await(ctx, run.ForeignID, run.RunID, StatusEnd)
	require.NoError(t, err)
}

func TestWorkflow_ScheduleFilterError(t *testing.T) {
	workflowName := "filter error test"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, run *workflow.Run[MyType, status]) (status, error) {
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
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(time.Millisecond),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	filterError := errors.New("filter error")
	filter := func(ctx context.Context) (bool, error) {
		return false, filterError
	}
	opt := workflow.WithScheduleFilter[MyType, status](filter)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("error-test", "@monthly", opt)
		require.NoError(t, err)
	}()
	wg.Wait()

	// Move to May 1st - first scheduled time
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place and expect it to fail due to filter error
	time.Sleep(50 * time.Millisecond)

	// Verify no record was created due to filter error
	_, err := recordStore.Latest(ctx, workflowName, "error-test")
	require.True(t, errors.Is(err, workflow.ErrRecordNotFound))
}

func TestWorkflow_ScheduleExistingRun(t *testing.T) {
	workflowName := "existing run test"
	b := workflow.NewBuilder[MyType, status](workflowName)
	b.AddStep(StatusStart, func(ctx context.Context, run *workflow.Run[MyType, status]) (status, error) {
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
		workflow.WithDefaultOptions(
			workflow.PollingFrequency(time.Millisecond),
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	// Create an initial run first and wait for completion
	_, err := wf.Trigger(ctx, "existing-test")
	require.NoError(t, err)

	firstRun, err := recordStore.Latest(ctx, workflowName, "existing-test")
	require.NoError(t, err)

	// Wait for the first run to complete
	_, err = wf.Await(ctx, firstRun.ForeignID, firstRun.RunID, StatusEnd)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		err := wf.Schedule("existing-test", "@monthly")
		require.NoError(t, err)
	}()
	wg.Wait()

	// Allow scheduler initialization
	time.Sleep(10 * time.Millisecond)

	// Move to May 1st - first scheduled time
	expectedTimestamp := time.Date(2023, time.May, 1, 0, 0, 0, 0, time.UTC)
	clock.SetTime(expectedTimestamp)

	// Allow scheduling to take place
	time.Sleep(200 * time.Millisecond)

	secondRun, err := recordStore.Latest(ctx, workflowName, "existing-test")
	require.NoError(t, err)

	// Should be a new run scheduled based on the existing run's timestamp
	require.NotEqual(t, firstRun.RunID, secondRun.RunID)
}
