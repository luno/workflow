package errorcounter_test

import (
	"testing"

	"github.com/luno/jettison/errors"
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
