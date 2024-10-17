package workflow_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/internal/metrics"
)

func runWorkflow(t *testing.T) *workflow.Workflow[string, status] {
	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return 0, nil
		},
		StatusMiddle,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 100),
	)

	clock := newClock()
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New(memrecordstore.WithClock(clock))
	w := b.Build(
		streamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithOutboxParallelCount(2),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	require.Nil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	require.Nil(t, err)

	err = update(ctx, recordStore, &workflow.Record{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		RunState:     workflow.RunStateRunning,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	require.Nil(t, err)

	// 1 hour = 3600 seconds
	clock.Step(time.Hour)

	w.Run(ctx)
	t.Cleanup(w.Stop)

	return w
}

func TestMetricProcessLag(t *testing.T) {
	metrics.ConsumerLag.Reset()

	runWorkflow(t)

	time.Sleep(time.Millisecond * 500)

	expected := `
# HELP workflow_process_lag_seconds lag between now and the current event timestamp in seconds
# TYPE workflow_process_lag_seconds gauge
workflow_process_lag_seconds{process_name="outbox-consumer-2-of-2",workflow_name="example"} 3600
workflow_process_lag_seconds{process_name="start-consumer-1-of-1",workflow_name="example"} 0
`

	err := testutil.CollectAndCompare(metrics.ConsumerLag, strings.NewReader(expected))
	require.Nil(t, err)

	metrics.ConsumerLag.Reset()
}

func TestMetricProcessLagAlert(t *testing.T) {
	metrics.ConsumerLagAlert.Reset()

	runWorkflow(t)

	time.Sleep(time.Millisecond * 750)

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.ConsumerLagAlert), 1)

	metrics.ConsumerLagAlert.Reset()
}

func TestMetricProcessStates(t *testing.T) {
	metrics.ProcessStates.Reset()

	w := runWorkflow(t)

	time.Sleep(time.Millisecond * 500)

	expected := `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="example-delete-consumer",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-1-of-2",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-2-of-2",workflow_name="example"} 2
workflow_process_states{process_name="start-consumer-1-of-1",workflow_name="example"} 2
`

	err := testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	require.Nil(t, err)

	w.Stop()

	// Ensure that the metrics are updated to false when stopping the process
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="example-delete-consumer",workflow_name="example"} 1
workflow_process_states{process_name="start-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-1-of-2",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-2-of-2",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	require.Nil(t, err)

	metrics.ProcessStates.Reset()
}

type mockScheduler struct {
	mu    sync.Mutex
	allow bool
}

func (m *mockScheduler) Await(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	for {
		m.mu.Lock()
		if m.allow {
			m.mu.Unlock()
			break
		}
		m.mu.Unlock()

		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		time.Sleep(time.Millisecond * 10)
	}

	ctx, cancel := context.WithCancel(ctx)
	return ctx, cancel, nil
}

var _ workflow.RoleScheduler = (*mockScheduler)(nil)

func TestMetricProcessIdleState(t *testing.T) {
	metrics.ProcessStates.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return 0, nil
		}, StatusMiddle,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 100),
	)
	b.AddStep(StatusMiddle,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return 0, nil
		}, StatusEnd,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 100),
	)

	clock := newClock()
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New()
	scheduler := &mockScheduler{}
	wf := b.Build(
		streamer,
		recordStore,
		scheduler,
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 250)

	// Ensure that the metrics are updated to idle before obtaining the role
	expected := `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="example-delete-consumer",workflow_name="example"} 3
workflow_process_states{process_name="start-consumer-1-of-1", workflow_name="example"} 3
workflow_process_states{process_name="middle-consumer-1-of-1", workflow_name="example"} 3
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 3
`

	err := testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	require.Nil(t, err)

	scheduler.mu.Lock()
	scheduler.allow = true
	scheduler.mu.Unlock()

	time.Sleep(time.Millisecond * 50)

	// Ensure that the metrics are updated to running obtaining the role
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="example-delete-consumer",workflow_name="example"} 2
workflow_process_states{process_name="middle-consumer-1-of-1",workflow_name="example"} 2
workflow_process_states{process_name="start-consumer-1-of-1",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 2
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	require.Nil(t, err)

	wf.Stop()

	// Ensure that the metrics are updated to shut down when processes are shutdown
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="example-delete-consumer",workflow_name="example"} 1
workflow_process_states{process_name="middle-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="start-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	require.Nil(t, err)

	metrics.ProcessStates.Reset()
}

func TestMetricProcessLatency(t *testing.T) {
	metrics.ProcessLatency.Reset()

	runWorkflow(t)

	time.Sleep(time.Millisecond * 500)

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.ProcessLatency), 2)

	metrics.ProcessLatency.Reset()
}

func TestMetricProcessErrors(t *testing.T) {
	metrics.ProcessErrors.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return 0, errors.New("mock error")
		}, StatusMiddle,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 100),
	)

	clock := newClock()
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New()
	wf := b.Build(
		streamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	require.Nil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	require.Nil(t, err)

	err = update(ctx, recordStore, &workflow.Record{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		RunState:     workflow.RunStateRunning,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	require.Nil(t, err)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 100)

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.ProcessErrors), 1)

	metrics.ProcessErrors.Reset()
}

func TestRunStateChanges(t *testing.T) {
	metrics.RunStateChanges.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return StatusMiddle, nil
		}, StatusMiddle,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 10),
	)
	b.AddStep(StatusMiddle,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return StatusEnd, nil
		}, StatusEnd,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 10),
	)

	w := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
		workflow.WithOutboxPollingFrequency(time.Millisecond),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	_, err := w.Trigger(ctx, "983467934", StatusStart)
	require.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.RunStateChanges), 3)

	metrics.RunStateChanges.Reset()
}

func TestMetricProcessSkippedEvents(t *testing.T) {
	metrics.ProcessSkippedEvents.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			return 0, nil
		}, StatusMiddle,
	).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 10),
	)

	w := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
		workflow.WithOutboxPollingFrequency(time.Millisecond),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	_, err := w.Trigger(ctx, "9834679343", StatusStart)
	require.Nil(t, err)

	_, err = w.Trigger(ctx, "2349839483", StatusStart)
	require.Nil(t, err)

	_, err = w.Trigger(ctx, "7548702398", StatusStart)
	require.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	require.GreaterOrEqual(t, testutil.CollectAndCount(metrics.ProcessSkippedEvents), 1)

	metrics.ProcessSkippedEvents.Reset()
}

func update(ctx context.Context, store workflow.RecordStore, wr *workflow.Record) error {
	return store.Store(ctx, wr, func(recordID int64) (workflow.OutboxEventData, error) {
		// Run ID would not have been set if it is a new record. Assign the recordID that the Store provides
		wr.ID = recordID
		return workflow.RecordToOutboxEventData(*wr, workflow.RunStateUnknown)
	})
}

func newClock() *clock_testing.FakeClock {
	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	return clock_testing.NewFakeClock(now)
}
