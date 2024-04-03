package workflow

import (
	"context"
	"io"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
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

func TestGraph(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	b.AddStep(statusMiddle, nil, statusEnd)
	wf := b.Build(nil, nil, nil, nil)

	expected := map[int][]int{
		int(statusStart): {
			int(statusMiddle),
		},
		int(statusMiddle): {
			int(statusEnd),
		},
	}

	require.Equal(t, expected, wf.graph)
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
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ErrBackOff(time.Minute))
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, time.Minute, wf.consumers[statusStart][0].errBackOff)
}

func TestWithParallelCount(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ParallelCount(100))
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, int(100), wf.consumers[statusStart][0].parallelCount)
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

	expected := map[int][]int{
		int(statusStart): {
			int(statusEnd),
		},
	}

	require.Equal(t, expected, wf.graph)
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
	wf := b.Build(nil, nil, nil, nil)

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
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, time.Minute, wf.timeouts[statusStart].pollingFrequency)
}

func TestConnectorConstruction(t *testing.T) {
	stream := &mockConsumer{}

	fn := func(ctx context.Context, w *Workflow[string, testStatus], e *Event) error {
		return nil
	}

	buidler := NewBuilder[string, testStatus]("workflow X")

	buidler.AddConnector(
		"my-test-connector",
		stream,
		fn,
		WithConnectorParallelCount(2),
	)

	workflowX := buidler.Build(nil, nil, nil, nil)

	for _, config := range workflowX.connectorConfigs {
		require.Equal(t, "my-test-connector", config.name)
		require.Equal(t, runtime.FuncForPC(reflect.ValueOf(stream).Pointer()).Name(), runtime.FuncForPC(reflect.ValueOf(config.consumerFn).Pointer()).Name())
		require.Equal(t, runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(), runtime.FuncForPC(reflect.ValueOf(config.connectorFn).Pointer()).Name())
		require.Equal(t, defaultErrBackOff, config.errBackOff)
		require.Equal(t, 2, config.parallelCount)
	}
}

type mockConsumer struct{}

func (m mockConsumer) Recv(ctx context.Context) (*Event, Ack, error) {
	return nil, nil, errors.New("not implemented")
}

func (m mockConsumer) Close() error {
	return errors.New("not implemented")
}

var _ Consumer = (*mockConsumer)(nil)

func TestWithStepLagAlert(t *testing.T) {
	testCases := []struct {
		name             string
		opts             []Option
		expectedLagAlert time.Duration
	}{
		{
			name:             "Ensure default lag alert is set by default",
			expectedLagAlert: defaultLagAlert,
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

			require.Equal(t, tc.expectedLagAlert, wf.consumers[statusStart][0].lagAlert)
		})
	}
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
	wf := b.Build(nil, nil, nil, nil)

	require.Equal(t, specifiedLag, wf.consumers[statusStart][0].lag)
}
