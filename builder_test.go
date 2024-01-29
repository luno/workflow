package workflow

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"
)

type testStatus int

const (
	statusUnknown testStatus = 1
	statusStart   testStatus = 1
	statusMiddle  testStatus = 2
	statusEnd     testStatus = 3
)

func (s testStatus) String() string {
	switch s {
	case statusStart:
		return "Start"
	case statusMiddle:
		return "Middle"
	case statusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

func TestDetermineEndPoints(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	b.AddStep(statusMiddle, nil, statusEnd)
	wf := b.Build(nil, nil, nil, nil)

	expected := map[testStatus]bool{
		statusEnd: true,
	}

	require.Equal(t, expected, wf.endPoints)
}

func TestWithStepErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle, WithStepErrBackOff(time.Minute))
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, time.Minute, wf.consumers[statusStart][0].ErrBackOff)
}

func TestStepDestinationStatus(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, statusMiddle, wf.consumers[statusStart][0].DestinationStatus)
}

func TestWithParallelCount(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle, WithParallelCount(100))
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, int(100), wf.consumers[statusStart][0].ParallelCount)
}

func TestWithClock(t *testing.T) {
	now := time.Now()
	clock := clock_testing.NewFakeClock(now)
	b := NewBuilder[string, testStatus]("determine starting points")
	wf := b.Build(nil, nil, nil, nil, WithClock(clock))

	clock.Step(time.Hour)

	require.Equal(t, now.Add(time.Hour), wf.clock.Now())
}

func TestAddingCallbacks(t *testing.T) {
	var exampleFn CallbackFunc[string, testStatus] = func(ctx context.Context, s *Record[string, testStatus], r io.Reader) (bool, error) {
		return true, nil
	}

	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddCallback(statusStart, exampleFn, statusEnd)
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, statusEnd, wf.callback[statusStart][0].DestinationStatus)
	require.NotNil(t, wf.callback[statusStart][0].CallbackFunc)
}

func TestWithTimeoutErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Record[string, testStatus], now time.Time) (bool, error) {
			return true, nil
		},
		statusEnd,
		WithTimeoutErrBackOff(time.Minute),
	)
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, time.Minute, wf.timeouts[statusStart].ErrBackOff)
}

func TestWithTimeoutPollingFrequency(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Record[string, testStatus], now time.Time) (bool, error) {
			return true, nil
		},
		statusEnd,
		WithTimeoutPollingFrequency(time.Minute),
	)
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, time.Minute, wf.timeouts[statusStart].PollingFrequency)
}

func TestWorkflowConnectorConstruction(t *testing.T) {
	externalStream := (EventStreamer)(nil)

	b := NewBuilder[string, testStatus]("workflow B")
	b.AddStep(statusStart, func(ctx context.Context, r *Record[string, testStatus]) (bool, error) {
		return true, nil
	}, statusMiddle)

	filter := func(ctx context.Context, e *Event) (string, error) {
		return e.Headers[HeaderWorkflowForeignID], nil
	}

	consumer := func(ctx context.Context, r *Record[string, testStatus], e *Event) (bool, error) {
		return true, nil
	}
	b.AddWorkflowConnector(
		WorkflowConnectionDetails{
			WorkflowName: "workflowA",
			Status:       9,
			Stream:       externalStream,
		},
		filter,
		statusMiddle,
		consumer,
		statusEnd,
		WithParallelCount(3),
		WithStepPollingFrequency(time.Second*10),
		WithStepErrBackOff(time.Minute),
	)
	wf := b.Build(nil, nil, nil, nil)

	for _, config := range wf.workflowConnectorConfigs {
		require.Equal(t, "workflowA", config.workflowName)
		require.Equal(t, 9, config.status)
		require.Equal(t, externalStream, config.stream)
		require.Equal(t, reflect.ValueOf(consumer).Pointer(), reflect.ValueOf(config.consumer).Pointer())
		require.Equal(t, statusEnd, config.to)
		require.Equal(t, time.Second*10, config.pollingFrequency)
		require.Equal(t, time.Minute, config.errBackOff)
		require.Equal(t, 3, config.parallelCount)
	}
}

func TestWithStepLagAlert(t *testing.T) {
	testCases := []struct {
		name             string
		opts             []StepOption
		expectedLagAlert time.Duration
	}{
		{
			name:             "Ensure default lag alert is set by default",
			expectedLagAlert: defaultLagAlert,
		},
		{
			name: "Ensure lag alert value is assigned",
			opts: []StepOption{
				WithStepLagAlert(time.Hour * 9),
			},
			expectedLagAlert: time.Hour * 9,
		},
		{
			name: "Ensure default lag alert is offset if accompanied with a step consumer lag",
			opts: []StepOption{
				WithStepConsumerLag(time.Hour),
			},
			expectedLagAlert: defaultLagAlert + time.Hour,
		},
		{
			name: "Ensure provided lag alert overrides default lag alert and is not offset when consumer lag is also present",
			opts: []StepOption{
				WithStepConsumerLag(time.Hour),
				WithStepLagAlert(time.Hour * 9),
			},
			expectedLagAlert: time.Hour * 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBuilder[string, testStatus]("consumer lag alert")
			b.AddStep(
				statusStart,
				func(ctx context.Context, r *Record[string, testStatus]) (bool, error) {
					return true, nil
				},
				statusEnd,
				tc.opts...,
			)
			wf := b.Build(nil, nil, nil, nil)

			require.Equal(t, tc.expectedLagAlert, wf.consumers[statusStart][0].LagAlert)
		})
	}
}

func TestWithStepConsumerLag(t *testing.T) {
	specifiedLag := time.Hour * 9
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Record[string, testStatus]) (bool, error) {
			return true, nil
		},
		statusEnd,
		WithStepConsumerLag(specifiedLag),
	)
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, specifiedLag, wf.consumers[statusStart][0].Lag)
}
