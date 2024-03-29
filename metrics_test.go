package workflow_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/jtest"
	"github.com/prometheus/client_golang/prometheus/testutil"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/internal/metrics"
)

func TestMetricProcessLag(t *testing.T) {
	metrics.ConsumerLag.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return false, nil
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New(memrecordstore.WithClock(clock))
	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	jtest.RequireNil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	jtest.RequireNil(t, err)

	err = update(ctx, recordStore, &workflow.WireRecord{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		IsStart:      true,
		IsEnd:        false,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	jtest.RequireNil(t, err)

	// 1 hour = 3600 seconds
	clock.Step(time.Hour)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 500)

	expected := `
# HELP workflow_process_lag_seconds lag between now and the current event timestamp in seconds
# TYPE workflow_process_lag_seconds gauge
workflow_process_lag_seconds{process_name="outbox-consumer-1-of-1",workflow_name="example"} 3600
workflow_process_lag_seconds{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 0
`

	err = testutil.CollectAndCompare(metrics.ConsumerLag, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	metrics.ConsumerLag.Reset()
}

func TestMetricProcessLagAlert(t *testing.T) {
	metrics.ConsumerLagAlert.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return false, nil
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New(memrecordstore.WithClock(clock))

	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	jtest.RequireNil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	jtest.RequireNil(t, err)

	err = update(ctx, recordStore, &workflow.WireRecord{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		IsStart:      true,
		IsEnd:        false,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	jtest.RequireNil(t, err)

	// 1 hour = 3600 seconds
	clock.Step(time.Hour)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 750)

	// We expect the "middle-to-end-consumer" to not be lagging as the event for that gets inserted only once we
	// consume the "start" event in the "start-to-middle-consumer".
	expected := `
# HELP workflow_process_lag_alert Whether or not the consumer lag crosses its alert threshold
# TYPE workflow_process_lag_alert gauge
workflow_process_lag_alert{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 0
workflow_process_lag_alert{process_name="outbox-consumer-1-of-1",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ConsumerLagAlert, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	metrics.ConsumerLagAlert.Reset()
}

func TestMetricProcessStates(t *testing.T) {
	metrics.ProcessStates.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusEnd, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New(memrecordstore.WithClock(clock))
	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithClock(clock),
		workflow.WithOutboxConfig(
			workflow.WithOutboxParallelCount(5),
		),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	jtest.RequireNil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	jtest.RequireNil(t, err)

	err = update(ctx, recordStore, &workflow.WireRecord{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		IsStart:      true,
		IsEnd:        false,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	jtest.RequireNil(t, err)

	// 1 hour = 3600 seconds
	clock.Step(time.Hour)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 500)

	expected := `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="start-to-middle-consumer-1-of-1", workflow_name="example"} 2
workflow_process_states{process_name="middle-to-end-consumer-1-of-1", workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-1-of-5",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-2-of-5",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-3-of-5",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-4-of-5",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-5-of-5",workflow_name="example"} 2
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	wf.Stop()

	// Ensure that the metrics are updated to false when stopping the process
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="middle-to-end-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-1-of-5",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-2-of-5",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-3-of-5",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-4-of-5",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-5-of-5",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	jtest.RequireNil(t, err)

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
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusEnd, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New()
	scheduler := &mockScheduler{}
	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
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
workflow_process_states{process_name="start-to-middle-consumer-1-of-1", workflow_name="example"} 3
workflow_process_states{process_name="middle-to-end-consumer-1-of-1", workflow_name="example"} 3
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 3
`

	err := testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	scheduler.mu.Lock()
	scheduler.allow = true
	scheduler.mu.Unlock()

	time.Sleep(time.Millisecond * 50)

	// Ensure that the metrics are updated to running obtaining the role
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="middle-to-end-consumer-1-of-1",workflow_name="example"} 2
workflow_process_states{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 2
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 2
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	wf.Stop()

	// Ensure that the metrics are updated to shutdown when processes are shutdown
	expected = `
# HELP workflow_process_states The current states of all the processes
# TYPE workflow_process_states gauge
workflow_process_states{process_name="middle-to-end-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 1
workflow_process_states{process_name="outbox-consumer-1-of-1",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ProcessStates, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	metrics.ProcessStates.Reset()
}

func TestMetricProcessLatency(t *testing.T) {
	metrics.ProcessLatency.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusEnd, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New(memrecordstore.WithClock(clock))
	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	jtest.RequireNil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	jtest.RequireNil(t, err)

	err = update(ctx, recordStore, &workflow.WireRecord{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		IsStart:      true,
		IsEnd:        false,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	jtest.RequireNil(t, err)

	clock.Step(time.Hour)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 500)

	// We expect the "middle-to-end-consumer" to not be lagging as the event for that gets inserted only once we
	// consume the "start" event in the "start-to-middle-consumer".
	expected := `
# HELP workflow_process_latency_seconds Event loop latency in seconds
# TYPE workflow_process_latency_seconds histogram
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="0.01"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="0.1"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="1"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="5"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="10"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="60"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="300"} 1
workflow_process_latency_seconds_bucket{process_name="start-to-middle-consumer-1-of-1",workflow_name="example",le="+Inf"} 1
workflow_process_latency_seconds_sum{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 0
workflow_process_latency_seconds_count{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="0.01"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="0.1"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="1"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="5"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="10"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="60"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="300"} 1
workflow_process_latency_seconds_bucket{process_name="middle-to-end-consumer-1-of-1",workflow_name="example",le="+Inf"} 1
workflow_process_latency_seconds_sum{process_name="middle-to-end-consumer-1-of-1",workflow_name="example"} 0
workflow_process_latency_seconds_count{process_name="middle-to-end-consumer-1-of-1",workflow_name="example"} 1
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="0.01"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="0.1"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="1"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="5"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="10"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="60"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="300"} 2
workflow_process_latency_seconds_bucket{process_name="outbox-consumer-1-of-1",workflow_name="example",le="+Inf"} 2
workflow_process_latency_seconds_sum{process_name="outbox-consumer-1-of-1",workflow_name="example"} 0
workflow_process_latency_seconds_count{process_name="outbox-consumer-1-of-1",workflow_name="example"} 2
`

	err = testutil.CollectAndCompare(metrics.ProcessLatency, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	metrics.ProcessLatency.Reset()
}

func TestMetricProcessErrors(t *testing.T) {
	metrics.ProcessErrors.Reset()

	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return false, errors.New("mock error")
	}, StatusMiddle, workflow.WithStepPollingFrequency(time.Millisecond*100))
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[string, status]) (bool, error) {
		return true, nil
	}, StatusEnd, workflow.WithStepPollingFrequency(time.Millisecond*100))

	nw := time.Now()
	now := time.Date(nw.Year(), nw.Month(), nw.Day(), nw.Hour(), 0, 0, 0, time.UTC)
	clock := clock_testing.NewFakeClock(now)
	streamer := memstreamer.New(memstreamer.WithClock(clock))
	recordStore := memrecordstore.New()
	wf := b.Build(
		streamer,
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithClock(clock),
	)

	ctx := context.Background()

	uid, err := uuid.NewUUID()
	jtest.RequireNil(t, err)

	runID := uid.String()

	var s string
	payload, err := workflow.Marshal(&s)
	jtest.RequireNil(t, err)

	err = update(ctx, recordStore, &workflow.WireRecord{
		WorkflowName: "example",
		ForeignID:    "29384723984732",
		RunID:        runID,
		Status:       int(StatusStart),
		IsStart:      true,
		IsEnd:        false,
		Object:       payload,
		CreatedAt:    clock.Now(),
	})
	jtest.RequireNil(t, err)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	time.Sleep(time.Millisecond * 100)

	// We expect the "middle-to-end-consumer" to not be lagging as the event for that gets inserted only once we
	// consume the "start" event in the "start-to-middle-consumer".
	expected := `
# HELP workflow_process_error_count Number of errors processing events
# TYPE workflow_process_error_count counter
workflow_process_error_count{process_name="start-to-middle-consumer-1-of-1",workflow_name="example"} 1
`

	err = testutil.CollectAndCompare(metrics.ProcessErrors, strings.NewReader(expected))
	jtest.RequireNil(t, err)

	metrics.ProcessErrors.Reset()
}

func TestMetricProcessSkippedEvents(t *testing.T) {}

func update(ctx context.Context, store workflow.RecordStore, wr *workflow.WireRecord) error {
	return store.Store(ctx, wr, func(recordID int64) (workflow.OutboxEventData, error) {
		// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
		wr.ID = recordID
		return workflow.WireRecordToOutboxEventData(*wr)
	})
}
