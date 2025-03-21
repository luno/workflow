package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/graph"
	internal_logger "github.com/luno/workflow/internal/logger"
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

	w := b.Build(nil, nil, nil)

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
	wf := b.Build(nil, nil, nil, WithDefaultOptions(PollingFrequency(time.Hour)))

	require.Equal(t, time.Minute, wf.consumers[statusStart].pollingFrequency)
}

func TestWithStepErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ErrBackOff(time.Minute))
	wf := b.Build(nil, nil, nil, WithDefaultOptions(ErrBackOff(time.Hour)))

	require.Equal(t, time.Minute, wf.consumers[statusStart].errBackOff)
}

func TestWithParallelCount(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle).WithOptions(ParallelCount(100))
	wf := b.Build(nil, nil, nil, WithDefaultOptions(ParallelCount(1)))

	require.Equal(t, int(100), wf.consumers[statusStart].parallelCount)
}

func TestWithClock(t *testing.T) {
	now := time.Now()
	clock := clock_testing.NewFakeClock(now)
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	wf := b.Build(nil, nil, nil, WithClock(clock))

	clock.Step(time.Hour)

	require.Equal(t, now.Add(time.Hour), wf.clock.Now())
}

func TestBuildOptions(t *testing.T) {
	now := time.Now()
	clock := clock_testing.NewFakeClock(now)
	timeoutStore := (TimeoutStore)(nil)
	logger := internal_logger.New(os.Stdout)
	opts := options{
		parallelCount:      2,
		pollingFrequency:   time.Millisecond,
		errBackOff:         time.Second,
		lag:                time.Minute,
		lagAlert:           time.Hour,
		customLagAlertSet:  true,
		pauseAfterErrCount: 3,
	}
	deleter := func(object *string) error {
		b := []byte(*object)
		*object = string(b[:len(b)/2])
		return nil
	}

	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddStep(statusStart, nil, statusMiddle)
	w := b.Build(
		nil,
		nil,
		nil,
		WithTimeoutStore(timeoutStore),
		WithClock(clock),
		WithDebugMode(),
		WithLogger(logger),
		WithDefaultOptions(
			ParallelCount(opts.parallelCount),
			PollingFrequency(opts.pollingFrequency),
			ErrBackOff(opts.errBackOff),
			ConsumeLag(opts.lag),
			LagAlert(opts.lagAlert),
			PauseAfterErrCount(opts.pauseAfterErrCount),
		),
		WithCustomDelete(deleter),
	)

	require.Equal(t, timeoutStore, w.timeoutStore)
	require.Equal(t, clock, w.clock)
	require.True(t, w.logger.debugMode)
	require.Equal(t, logger, w.logger.inner)
	require.Equal(t, opts, w.defaultOpts)
	require.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(w.customDelete).Pointer()).Name(), "github.com/luno/workflow.TestBuildOptions.WithCustomDelete"))
	object, err := w.customDelete(&Record{
		Object: []byte(`"hello world"`),
	})
	require.NoError(t, err)
	require.Equal(t, `"hello"`, string(object))
}

func TestAddingCallbacks(t *testing.T) {
	var exampleFn CallbackFunc[string, testStatus] = func(ctx context.Context, s *Run[string, testStatus], r io.Reader) (testStatus, error) {
		return statusEnd, nil
	}

	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddCallback(statusStart, exampleFn, statusEnd)
	wf := b.Build(nil, nil, nil)

	require.NotNil(t, wf.callback[statusStart][0].CallbackFunc)
}

func TestAddTimeoutErrBackOff(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Run[string, testStatus], now time.Time) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		ErrBackOff(time.Minute),
	)

	require.Equal(t, time.Minute, b.workflow.timeouts[statusStart].errBackOff)
}

func TestAddTimeoutPollingFrequency(t *testing.T) {
	b := NewBuilder[string, testStatus]("determine starting points")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, t *Run[string, testStatus], now time.Time) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		PollingFrequency(time.Minute),
	)

	require.Equal(t, time.Minute, b.workflow.timeouts[statusStart].pollingFrequency)
}

func TestDefaultStartingPoint(t *testing.T) {
	t.Run("Default starting point defaults to first node", func(t *testing.T) {
		b := NewBuilder[string, testStatus]("consumer lag")
		b.AddStep(
			statusStart,
			func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
				return statusEnd, nil
			},
			statusMiddle,
		)
		b.AddStep(
			statusMiddle,
			func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
				return statusEnd, nil
			},
			statusEnd,
		)
		wf := b.Build(nil, nil, nil)

		require.Equal(t, statusStart, wf.defaultStartingPoint)
	})

	t.Run("Require starting point", func(t *testing.T) {
		require.PanicsWithValue(t,
			"Workflow requires at least one starting point. Please provide at least one Step, Callback, or Timeout to add a starting point.",
			func() {
				b := NewBuilder[string, testStatus]("")
				_ = b.Build(nil, nil, nil)
			},
		)
	})
}

func TestAddTimeoutDontAllowParallelCount(t *testing.T) {
	require.PanicsWithValue(t,
		"Cannot configure parallel timeout",
		func() {
			b := NewBuilder[string, testStatus]("")
			b.AddTimeout(statusStart, nil, nil, statusEnd).WithOptions(
				ParallelCount(2),
			)
		},
	)
}

func TestAddTimeoutDontAllowLag(t *testing.T) {
	require.PanicsWithValue(t,
		"Cannot configure lag for timeout",
		func() {
			b := NewBuilder[string, testStatus]("")
			b.AddTimeout(statusStart, nil, nil, statusEnd).WithOptions(
				ConsumeLag(time.Hour),
			)
		},
	)
}

func TestConnectorConstruction(t *testing.T) {
	fn := func(ctx context.Context, w API[string, testStatus], e *ConnectorEvent) error {
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

	b.AddStep(statusStart, nil, statusEnd)
	w := b.Build(nil, nil, nil)

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
				func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
					return statusEnd, nil
				},
				statusEnd,
			).WithOptions(
				tc.opts...,
			)

			wf := b.Build(nil, nil, nil)

			require.Equal(t, tc.expectedLagAlert, wf.consumers[statusStart].lagAlert)
		})
	}
}

func TestAddStepSingleUseValidation(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
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
				func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
					return statusEnd, nil
				},
				statusEnd,
			)
		}, "Adding duplicate step should panic")
}

func TestConfigureTimeoutWithoutTimeoutStore(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddTimeout(
		statusStart,
		DurationTimerFunc[string, testStatus](time.Hour),
		func(ctx context.Context, r *Run[string, testStatus], now time.Time) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)

	// Should panic as setting a second config of statusStart
	require.PanicsWithValue(t,
		"Cannot configure timeouts without providing TimeoutStore for workflow",
		func() {
			b.Build(
				nil,
				nil,
				nil,
			)
		}, "Adding a timout step without providing a timeout store should panic")
}

func TestWithStepConsumerLag(t *testing.T) {
	specifiedLag := time.Hour * 9
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	).WithOptions(
		ConsumeLag(specifiedLag),
	)
	wf := b.Build(nil, nil, nil, WithDefaultOptions(ConsumeLag(time.Minute)))

	require.Equal(t, specifiedLag, wf.consumers[statusStart].lag)
}

func TestWithDefaultOptions(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)
	wf := b.Build(
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

func TestWithOutboxOptions(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)
	wf := b.Build(
		nil,
		nil,
		nil,
		WithOutboxOptions(
			OutboxPollingFrequency(time.Hour),
			OutboxErrBackOff(2*time.Hour),
			OutboxLagAlert(3*time.Hour),
			OutboxLookupLimit(4),
		),
	)

	require.Equal(t, time.Hour, wf.outboxConfig.pollingFrequency)
	require.Equal(t, 2*time.Hour, wf.outboxConfig.errBackOff)
	require.Equal(t, 3*time.Hour, wf.outboxConfig.lagAlert)
	require.Equal(t, int64(4), wf.outboxConfig.limit)
}

func TestWithPauseAutoRetry(t *testing.T) {
	b := NewBuilder[string, testStatus]("consumer lag")
	b.AddStep(
		statusStart,
		func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
			return statusEnd, nil
		},
		statusEnd,
	)
	wf := b.Build(
		nil,
		nil,
		nil,
		WithPauseRetry(time.Minute),
	)

	require.True(t, wf.pausedRecordsRetry.enabled)
	require.Equal(t, time.Minute, wf.pausedRecordsRetry.resumeAfter)
}

func TestConnectorNamesAreUnique(t *testing.T) {
	require.PanicsWithValue(t,
		"connector names need to be unique",
		func() {
			b := NewBuilder[string, testStatus]("workflow X")
			b.AddConnector("my-test-connector", nil, nil)
			b.AddConnector("my-test-connector", nil, nil)
		},
	)
}

func TestOnPause(t *testing.T) {
	b := NewBuilder[string, testStatus]("")
	testErr := errors.New("test error")
	fn := func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
		return testErr
	}
	b.OnPause(fn)
	actualFn, ok := b.workflow.runStateChangeHooks[RunStatePaused]
	require.True(t, ok)
	require.Equal(t, testErr, actualFn(nil, nil))
}

func TestOnCancel(t *testing.T) {
	b := NewBuilder[string, testStatus]("")
	testErr := errors.New("test error")
	fn := func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
		return testErr
	}
	b.OnCancel(fn)
	actualFn, ok := b.workflow.runStateChangeHooks[RunStateCancelled]
	require.True(t, ok)
	require.Equal(t, testErr, actualFn(nil, nil))
}

func TestOnComplete(t *testing.T) {
	b := NewBuilder[string, testStatus]("")
	testErr := errors.New("test error")
	fn := func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
		return testErr
	}
	b.OnComplete(fn)
	actualFn, ok := b.workflow.runStateChangeHooks[RunStateCompleted]
	require.True(t, ok)
	require.Equal(t, testErr, actualFn(nil, nil))
}
