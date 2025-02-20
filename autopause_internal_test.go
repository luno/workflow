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
		nw := time.Now()
		clock := clock_testing.NewFakeClock(nw)
		retryInterval := 20 * time.Minute
		recordList := []Record{
			{
				RunID:     "A",
				RunState:  RunStatePaused,
				UpdatedAt: clock.Now().Add(-time.Hour),
			},
			{
				RunID:     "B",
				RunState:  RunStatePaused,
				UpdatedAt: clock.Now().Add(-time.Hour * 48),
			},
			{
				RunID:     "C",
				RunState:  RunStatePaused,
				UpdatedAt: clock.Now().Add(-time.Second),
			},
			{
				RunID:     "D",
				RunState:  RunStatePaused,
				UpdatedAt: clock.Now().Add(-retryInterval),
			},
			{
				RunID:     "E",
				RunState:  RunStatePaused,
				UpdatedAt: clock.Now().Add(-time.Minute * 10),
			},
		}
		updateCounter := make(map[string]int)
		err := retryPausedRecords(
			ctx,
			"example workflow",
			func(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error) {
				filter := MakeFilter(filters...)
				require.True(t, filter.byRunState.Enabled)
				require.Equal(t, "3", filter.byRunState.Value)
				require.Equal(t, int64(0), offsetID)
				require.Equal(t, 100, limit)
				require.Equal(t, OrderTypeAscending, order)
				return recordList, nil
			},
			func(ctx context.Context, record *Record) error {
				// Require that the record is updated to RunStateRunning
				require.Equal(t, RunStateRunning, record.RunState)
				updateCounter[record.RunID] += 1
				return nil
			},
			clock,
			"process-name",
			100,
			retryInterval,
		)
		require.NoError(t, err)

		require.Equal(t, map[string]int{
			"A": 1,
			"B": 1,
			"D": 1,
		}, updateCounter)
	})

	t.Run("Returns error from listing records failure", func(t *testing.T) {
		testErr := errors.New("test error")
		err := retryPausedRecords(
			context.Background(),
			"example workflow",
			func(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error) {
				return nil, testErr
			},
			nil,
			&clock_testing.FakeClock{},
			"process-name",
			100,
			time.Minute,
		)
		require.Error(t, err, testErr)
	})

	t.Run("Returns error when updating record to Running fails", func(t *testing.T) {
		testErr := errors.New("test error")
		clock := clock_testing.NewFakeClock(time.Now())
		err := retryPausedRecords(
			context.Background(),
			"example workflow",
			func(ctx context.Context, workflowName string, offsetID int64, limit int, order OrderType, filters ...RecordFilter) ([]Record, error) {
				return []Record{
					{
						RunState:  RunStatePaused,
						UpdatedAt: clock.Now().Add(-time.Hour),
					},
				}, nil
			},
			func(ctx context.Context, record *Record) error {
				return testErr
			},
			clock,
			"process-name",
			100,
			time.Minute,
		)
		require.Error(t, err, testErr)
	})
}
