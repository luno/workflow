package errorcounter_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/errorcounter"
)

func TestErrorCounter(t *testing.T) {
	t.Run("Add 3 and get 3", func(t *testing.T) {
		c := errorcounter.New()
		err := errors.New("test error")

		c.Add(err, "label 1", "label 2")
		c.Add(err, "label 1", "label 2")
		count := c.Add(err, "label 1", "label 2")
		require.Equal(t, 3, count)

		require.Equal(t, 3, c.Count(err, "label 1", "label 2"))

		c.Clear(err, "label 1", "label 2")
		require.Equal(t, 0, c.Count(err, "label 1", "label 2"))
	})

	t.Run("Single label", func(t *testing.T) {
		c := errorcounter.New()
		err := errors.New("test error")

		c.Add(err, "only-label")
		count := c.Add(err, "only-label")
		require.Equal(t, 2, count)

		require.Equal(t, 2, c.Count(err, "only-label"))

		c.Clear(err, "only-label")
		require.Equal(t, 0, c.Count(err, "only-label"))
	})

	t.Run("Add 0 and get 0", func(t *testing.T) {
		c := errorcounter.New()
		require.Equal(t, 0, c.Count(errors.New("test error"), "label 1"))
	})
}

func TestErrorCounter_DifferentErrorsSameLabels(t *testing.T) {
	c := errorcounter.New()

	// Different error messages with the same labels should share a counter.
	c.Add(errors.New("connection refused at 10:00:01"), "processName", "run-123")
	c.Add(errors.New("connection refused at 10:00:02"), "processName", "run-123")
	count := c.Add(errors.New("timeout after 30s"), "processName", "run-123")

	require.Equal(t, 3, count)

	// Count should work regardless of which error is passed.
	require.Equal(t, 3, c.Count(errors.New("completely different error"), "processName", "run-123"))
}

func TestErrorCounter_ClearRemovesKey(t *testing.T) {
	c := errorcounter.New()

	c.Add(errors.New("err"), "process", "run-1")
	c.Add(errors.New("err"), "process", "run-1")
	require.Equal(t, 2, c.Count(errors.New("err"), "process", "run-1"))

	c.Clear(errors.New("err"), "process", "run-1")

	// After clear, count should be 0 and next Add should return 1.
	require.Equal(t, 0, c.Count(errors.New("err"), "process", "run-1"))
	require.Equal(t, 1, c.Add(errors.New("err"), "process", "run-1"))
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
