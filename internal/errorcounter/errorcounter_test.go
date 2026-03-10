package errorcounter_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/errorcounter"
)

func TestErrorCounter(t *testing.T) {
	testCases := []struct {
		name           string
		inputErr       error
		labels         []string
		iterationCount int
		expectedCount  int
	}{
		{
			name:           "Add 3 and get 3",
			inputErr:       errors.New("test error"),
			labels:         []string{"label 1", "label 2"},
			iterationCount: 3,
			expectedCount:  3,
		},
		{
			name:           "Add 1 and get 1 - no labels",
			inputErr:       errors.New("test error"),
			labels:         []string{},
			iterationCount: 3,
			expectedCount:  3,
		},
		{
			name:           "Add 0 and get 0",
			inputErr:       errors.New("test error"),
			labels:         []string{"label 1"},
			iterationCount: 0,
			expectedCount:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := errorcounter.New()

			var currentCount int
			for i := 0; i < tc.iterationCount; i++ {
				currentCount = c.Add(tc.inputErr, tc.labels...)
			}
			require.Equal(t, tc.expectedCount, currentCount)

			count := c.Count(tc.inputErr, tc.labels...)
			require.Equal(t, tc.expectedCount, count)

			c.Clear(tc.inputErr, tc.labels...)
			count = c.Count(tc.inputErr, tc.labels...)
			require.Equal(t, 0, count)
		})
	}
}

func TestErrorCounter_DifferentErrorsSameLabels(t *testing.T) {
	c := errorcounter.New()
	labels := []string{"processName", "run-123"}

	// Different error messages with the same labels should share a counter.
	c.Add(errors.New("connection refused at 10:00:01"), labels...)
	c.Add(errors.New("connection refused at 10:00:02"), labels...)
	count := c.Add(errors.New("timeout after 30s"), labels...)

	require.Equal(t, 3, count)

	// Count should work regardless of which error is passed.
	require.Equal(t, 3, c.Count(errors.New("completely different error"), labels...))
}

func TestErrorCounter_ClearRemovesKey(t *testing.T) {
	c := errorcounter.New()
	labels := []string{"process", "run-1"}

	c.Add(errors.New("err"), labels...)
	c.Add(errors.New("err"), labels...)
	require.Equal(t, 2, c.Count(errors.New("err"), labels...))

	c.Clear(errors.New("err"), labels...)

	// After clear, count should be 0 and next Add should return 1.
	require.Equal(t, 0, c.Count(errors.New("err"), labels...))
	require.Equal(t, 1, c.Add(errors.New("err"), labels...))
}

func TestErrorCounter_DifferentLabelsSeparateCounters(t *testing.T) {
	c := errorcounter.New()
	err := errors.New("same error")

	c.Add(err, "process-a", "run-1")
	c.Add(err, "process-a", "run-1")
	c.Add(err, "process-b", "run-2")

	require.Equal(t, 2, c.Count(err, "process-a", "run-1"))
	require.Equal(t, 1, c.Count(err, "process-b", "run-2"))
}
