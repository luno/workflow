package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/errorcounter"
)

func Test_maybeAutoPause(t *testing.T) {
	ctx := context.Background()
	counter := errorcounter.New()
	testErr := errors.New("test error")
	pauseErr := errors.New("pause error")
	processName := "process"

	testCases := []struct {
		name             string
		countBeforePause int
		pausesRecord     bool
		errCount         int
		expectedErr      error
		pauseFn          func(ctx context.Context) error
	}{
		{
			name:             "Default - not configured, no previous errors - does not pause workflow run",
			countBeforePause: 0,
			pausesRecord:     false,
			errCount:         0,
		},
		{
			name:             "Default - not configured, has previous errors - does not pause workflow run",
			countBeforePause: 0,
			pausesRecord:     false,
			errCount:         100,
		},
		{
			name:             "Pause record that has max retry of 1",
			countBeforePause: 1,
			pausesRecord:     true,
			errCount:         1,
		},
		{
			name:             "Do not pause record that has not passed max retry",
			countBeforePause: 2,
			pausesRecord:     false,
			errCount:         1,
		},
		{
			name:             "Return error when pause fails",
			countBeforePause: 1,
			pausesRecord:     false,
			errCount:         1,
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

			t.Log(counter.Count(testErr, processName, r.RunID))

			paused, err := maybePause(
				ctx,
				tc.countBeforePause,
				counter,
				testErr,
				processName,
				&r,
				&logger{},
			)
			require.ErrorIs(t, err, tc.expectedErr)
			require.Equal(t, tc.pausesRecord, paused)
		})
	}
}
