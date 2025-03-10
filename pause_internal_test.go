package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow/internal/errorcounter"
)

func Test_maybeAutoPause(t *testing.T) {
	ctx := context.Background()
	counter := errorcounter.New()
	testErr := errors.New("test error")
	pauseErr := errors.New("pause error")
	processName := "process"

	testCases := []struct {
		name               string
		pauseAfterErrCount int
		pausesRecord       bool
		errCount           int
		expectedErr        error
		pauseFn            func(ctx context.Context) error
	}{
		{
			name:               "Default - not configured, no previous errors - does not pause workflow run",
			pauseAfterErrCount: 0,
			pausesRecord:       false,
			errCount:           0,
		},
		{
			name:               "Default - not configured, has previous errors - does not pause workflow run",
			pauseAfterErrCount: 0,
			pausesRecord:       false,
			errCount:           100,
		},
		{
			name:               "Pause record that has max retry of 1",
			pauseAfterErrCount: 1,
			pausesRecord:       true,
			errCount:           1,
		},
		{
			name:               "Do not pause record that has not passed max retry",
			pauseAfterErrCount: 3,
			pausesRecord:       false,
			errCount:           1,
		},
		{
			name:               "Return error when pause fails",
			pauseAfterErrCount: 1,
			pausesRecord:       false,
			errCount:           1,
			pauseFn: func(ctx context.Context) error {
				return pauseErr
			},
			expectedErr: pauseErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewTestingRun[string, testStatus](t, Record{
				RunID: "run-id",
			}, "test", WithPauseFn(tc.pauseFn))

			counter.Clear(testErr, processName, r.RunID)
			for range tc.errCount {
				counter.Add(testErr, processName, r.RunID)
			}

			paused, err := maybePause(
				ctx,
				tc.pauseAfterErrCount,
				counter,
				testErr,
				processName,
				r,
				&logger{},
			)
			require.ErrorIs(t, err, tc.expectedErr)
			require.Equal(t, tc.pausesRecord, paused)
		})
	}
}

func Test_retryPausedRecords(t *testing.T) {
	t.Run("Golden path", func(t *testing.T) {
		ctx := context.Background()
		clock := clock_testing.NewFakeClock(time.Now())
		var calledUpdate bool
		err := autoRetryConsumer(
			func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					RunState:  RunStatePaused,
					UpdatedAt: clock.Now().Add(-time.Hour),
				}, nil
			},
			func(ctx context.Context, record *Record) error {
				calledUpdate = true
				require.Equal(t, RunStateRunning, record.RunState)
				return nil
			},
			clock,
			time.Hour,
		)(ctx, &Event{})
		require.NoError(t, err)
		require.True(t, calledUpdate)
	})

	t.Run("Return non-nil error on lookup", func(t *testing.T) {
		ctx := context.Background()
		clock := clock_testing.NewFakeClock(time.Now())
		testErr := errors.New("test error")
		err := autoRetryConsumer(
			func(ctx context.Context, runID string) (*Record, error) {
				return nil, testErr
			},
			nil,
			clock,
			time.Hour,
		)(ctx, &Event{})
		require.Error(t, testErr, err)
	})

	t.Run("Return non-nil error from store func", func(t *testing.T) {
		ctx := context.Background()
		clock := clock_testing.NewFakeClock(time.Now())
		testErr := errors.New("test error")
		err := autoRetryConsumer(
			func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					RunState:  RunStatePaused,
					CreatedAt: clock.Now(),
				}, nil
			},
			func(ctx context.Context, record *Record) error {
				return testErr
			},
			clock,
			time.Hour,
		)(ctx, &Event{})
		require.Error(t, testErr, err)
	})

	t.Run("Golden path - return nil with no update if before resume time", func(t *testing.T) {
		ctx := context.Background()
		clock := clock_testing.NewFakeClock(time.Now())
		var calledUpdate bool
		err := autoRetryConsumer(
			func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					RunState:  RunStatePaused,
					UpdatedAt: clock.Now(),
				}, nil
			},
			func(ctx context.Context, record *Record) error {
				calledUpdate = true
				return nil
			},
			clock,
			time.Hour,
		)(ctx, &Event{})
		require.NoError(t, err)
		require.False(t, calledUpdate)
	})

	t.Run("Not in paused state", func(t *testing.T) {
		ctx := context.Background()
		clock := clock_testing.NewFakeClock(time.Now())
		var calledUpdate bool
		err := autoRetryConsumer(
			func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					RunState:  RunStateCompleted,
					UpdatedAt: clock.Now().Add(-time.Hour),
				}, nil
			},
			func(ctx context.Context, record *Record) error {
				calledUpdate = true
				return nil
			},
			clock,
			time.Hour,
		)(ctx, &Event{})
		require.NoError(t, err)
		require.False(t, calledUpdate)
	})
}
