package workflow

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
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

func TestStatusGraph(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	b.AddCallback(statusMiddle, nil, statusEnd)

	w := b.Build(nil, nil, nil, nil)

	info := w.statusGraph.Info()
	expectedTransitions := []graph.Transition{
		{
			From: int(statusStart),
			To:   int(statusMiddle),
		},
		{
			From: int(statusMiddle),
			To:   int(statusEnd),
		},
	}

	require.Equal(t, expectedTransitions, info.Transitions)

	expectedStarting := []int{
		int(statusStart),
	}

	require.Equal(t, expectedStarting, info.StartingNodes)

	expectedTerminal := []int{
		int(statusEnd),
	}

	require.Equal(t, expectedTerminal, info.TerminalNodes)
}

func TestWithStepPollingFrequency(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(PollingFrequency(time.Minute))
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(PollingFrequency(time.Hour)))

	require.Equal(t, time.Minute, wf.consumers[statusStart].pollingFrequency)
}

func TestWithStepErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ErrBackOff(time.Minute))
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(ErrBackOff(time.Hour)))

	require.Equal(t, time.Minute, wf.consumers[statusStart].errBackOff)
}

func TestWithParallelCount(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ParallelCount(100))
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(ParallelCount(1)))

	require.Equal(t, int(100), wf.consumers[statusStart].parallelCount)
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
	var exampleFn CallbackFunc[string, testStatus] = func(ctx context.Context, s *Record[string, testStatus], r io.Reader) (testStatus, error) {
		return statusEnd, nil
	}

	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddCallback(statusStart, exampleFn, statusEnd)
	wf := b.Build(nil, nil, nil, nil)

	require.NotNil(t, wf.callback[statusStart][0].CallbackFunc)
}

func TestWithTimeoutErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Record[string, testStatus], now time.Time) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		ErrBackOff(time.Minute),
	)
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(ConsumeLag(time.Hour)))

	require.Equal(t, time.Minute, wf.timeouts[statusStart].errBackOff)
}

func TestWithTimeoutPollingFrequency(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Record[string, testStatus], now time.Time) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		PollingFrequency(time.Minute),
	)
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(PollingFrequency(time.Hour)))

	require.Equal(t, time.Minute, wf.timeouts[statusStart].pollingFrequency)
}

func TestConnectorConstruction(t *testing.T) {
	fn := func(ctx context.Context, w *Workflow[string, testStatus], e *ConnectorEvent) error {
		return nil
	}

	connector := &mockConnector{}

	b := NewBuilder[string, testStatus]("workflow X")
	b.AddConnector(
		"my-test-connector",
		connector,
		fn,
	).WithOptions(
		ParallelCount(2),
		ErrBackOff(time.Hour*6),
	)

	w := b.Build(nil, nil, nil, nil)

	for _, config := range w.connectorConfigs {
		require.Equal(t, "my-test-connector", config.name)
		require.NotEmpty(t, runtime.FuncForPC(reflect.ValueOf(config.connectorFn).Pointer()).Name())
		require.Equal(t, runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(), runtime.FuncForPC(reflect.ValueOf(config.connectorFn).Pointer()).Name())
		require.Equal(t, time.Hour*6, config.errBackOff)
		require.Equal(t, 2, config.parallelCount)
	}
}

type mockConnector struct{}

func (mc mockConnector) Make(ctx context.Context, consumerName string) (ConnectorConsumer, error) {
	return &mockExternalConsumer{}, nil
}

type mockExternalConsumer struct{}

func (m mockExternalConsumer) Recv(ctx context.Context) (*ConnectorEvent, Ack, error) {
	return nil, nil, errors.New("not implemented")
}

func (m mockExternalConsumer) Close() error {
	return errors.New("not implemented")
}

func TestWithStepLagAlert(t *testing.T) {
	testCases := []struct {
		name             string
		opts             []Option
		expectedLagAlert time.Duration
	}{
		{
			name:             "No lag alert",
			expectedLagAlert: 0,
		},
		{
			name: "Ensure lag alert value is assigned",
			opts: []Option{
				LagAlert(time.Hour * 9),
			},
			expectedLagAlert: time.Hour * 9,
		},
		{
			name: "Ensure default lag alert is offset if accompanied with a step consumer lag",
			opts: []Option{
				ConsumeLag(time.Hour),
			},
			expectedLagAlert: defaultLagAlert + time.Hour,
		},
		{
			name: "Ensure provided lag alert overrides default lag alert and is not offset when consumer lag is also present",
			opts: []Option{
				ConsumeLag(time.Hour),
				LagAlert(time.Hour * 9),
			},
			expectedLagAlert: time.Hour * 9,
		},
		{
			name: "Ensure order provided doesnt impact override lag alert",
			opts: []Option{
				LagAlert(time.Hour * 9),
				ConsumeLag(time.Hour),
			},
			expectedLagAlert: time.Hour * 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBuilder[string, testStatus]("consumer lag alert")
			b.AddStep(
				statusStart,
				func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
					return statusEnd, nil
				},
				statusEnd,
			).WithOptions(
				tc.opts...,
			)

			wf := b.Build(nil, nil, nil, nil)

			require.Equal(t, tc.expectedLagAlert, wf.consumers[statusStart].lagAlert)
		})
	}
}

func TestAddStepSingleUseValidation(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)

	// Should panic as setting a second config of statusStart
	require.PanicsWithValue(t,
		fmt.Sprintf("'AddStep(%v,' already exists. Only one Step can be configured to consume the status", statusStart.String()),
		func() {
			b.AddStep(
				statusStart,
				func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
					return statusEnd, nil
				},
				statusEnd,
			)
		}, "Adding duplicate step should panic")
}

func TestWithStepConsumerLag(t *testing.T) {
	specifiedLag := time.Hour * 9
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		ConsumeLag(specifiedLag),
	)
	wf := b.Build(nil, nil, nil, nil, WithDefaultOptions(ConsumeLag(time.Minute)))

	require.Equal(t, specifiedLag, wf.consumers[statusStart].lag)
}

func TestWithDefaultOptions(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Record[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)
	wf := b.Build(
		nil,
		nil,
		nil,
		nil,
		WithDefaultOptions(
			PollingFrequency(time.Hour),
			ErrBackOff(3*time.Hour),
			LagAlert(4*time.Hour),
			ParallelCount(5),
			ConsumeLag(6*time.Hour),
			PauseAfterErrCount(700),
		),
	)

	require.Equal(t, time.Hour, wf.defaultOpts.pollingFrequency)
	require.Equal(t, 3*time.Hour, wf.defaultOpts.errBackOff)
	require.Equal(t, 4*time.Hour, wf.defaultOpts.lagAlert)
	require.Equal(t, 5, wf.defaultOpts.parallelCount)
	require.Equal(t, 6*time.Hour, wf.defaultOpts.lag)
	require.Equal(t, 700, wf.defaultOpts.pauseAfterErrCount)
}
