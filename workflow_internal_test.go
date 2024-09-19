package workflow

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"
	clock_testing "k8s.io/utils/clock/testing"

	internal_logger "github.com/luno/workflow/internal/logger"
)

func Test_runOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("Returns non-nil error (context.Canceled) when parent ctx is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			nil,
			nil,
			nil,
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Returns awaitRole's context cancellation", func(t *testing.T) {
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				return nil, nil, context.Canceled
			},
			func(ctx context.Context) error {
				return nil
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Retry awaitRole error that is not context.Canceled", func(t *testing.T) {
		testErr := errors.New("test error")
		buf := bytes.NewBuffer([]byte{})
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				return nil, nil, testErr
			},
			func(ctx context.Context) error {
				return nil
			},
			&logger{
				debugMode: false,
				inner:     internal_logger.New(buf),
			},
			clock.RealClock{},
			time.Minute,
		)
		require.Nil(t, err)
		require.Contains(t, buf.String(), `"msg":"run error [role=role-1]: test error"`)
	})

	t.Run("Cancelled parent context during process execution retries and exits with context.Canceled ", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				// Cancel parent context and return context error
				cancel()
				return ctx.Err()
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		// If the err is nil then it will be retried
		require.Nil(t, err)

		// Context has been cancelled previously in the last runOnce so we expect
		// this to immediately return context.Canceled and need any parameters.
		err = runOnce(
			ctx,
			"",
			"",
			"",
			nil,
			nil,
			nil,
			nil,
			nil,
			time.Minute,
		)
		require.True(t, errors.Is(err, context.Canceled))
	})

	t.Run("Retry process error that is not context.Canceled", func(t *testing.T) {
		testErr := errors.New("test error")
		buf := bytes.NewBuffer([]byte{})
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				return testErr
			},
			&logger{
				debugMode: false,
				inner:     internal_logger.New(buf),
			},
			clock.RealClock{},
			time.Millisecond,
		)

		require.Nil(t, err)
		require.Contains(t, buf.String(), `"msg":"run error [role=role-1]: test error`)
	})

	t.Run("Updates process state", func(t *testing.T) {
		var stateChanges []string
		err := runOnce(
			ctx,
			"workflow-1",
			"role-1",
			"process-1",
			func(processName string, s State) {
				stateChanges = append(stateChanges, s.String())
			},
			func(ctx context.Context, role string) (context.Context, context.CancelFunc, error) {
				ctx, cancel := context.WithCancel(ctx)
				return ctx, cancel, nil
			},
			func(ctx context.Context) error {
				return nil
			},
			nil,
			clock_testing.NewFakeClock(time.Now()),
			time.Minute,
		)
		require.Nil(t, err)

		expected := []string{StateIdle.String(), StateRunning.String()}
		require.Equal(t, expected, stateChanges)
	})
}
