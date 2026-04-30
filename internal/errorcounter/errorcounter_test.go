package errorcounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/errorcounter"
)

func TestErrorCounter(t *testing.T) {
	t.Run("Add 3 and get 3", func(t *testing.T) {
		c := errorcounter.New()

		c.Add("label 1", "label 2")
		c.Add("label 1", "label 2")
		count := c.Add("label 1", "label 2")
		require.Equal(t, 3, count)

		require.Equal(t, 3, c.Count("label 1", "label 2"))

		c.Clear("label 1", "label 2")
		require.Equal(t, 0, c.Count("label 1", "label 2"))
	})

	t.Run("Single label", func(t *testing.T) {
		c := errorcounter.New()

		c.Add("only-label")
		count := c.Add("only-label")
		require.Equal(t, 2, count)

		require.Equal(t, 2, c.Count("only-label"))

		c.Clear("only-label")
		require.Equal(t, 0, c.Count("only-label"))
	})

	t.Run("Add 0 and get 0", func(t *testing.T) {
		c := errorcounter.New()
		require.Equal(t, 0, c.Count("label 1"))
	})
}

func TestErrorCounter_ClearRemovesKey(t *testing.T) {
	c := errorcounter.New()

	c.Add("process", "run-1")
	c.Add("process", "run-1")
	require.Equal(t, 2, c.Count("process", "run-1"))

	c.Clear("process", "run-1")

	// After clear, count should be 0 and next Add should return 1.
	require.Equal(t, 0, c.Count("process", "run-1"))
	require.Equal(t, 1, c.Add("process", "run-1"))
}

func TestErrorCounter_DifferentLabelsSeparateCounters(t *testing.T) {
	c := errorcounter.New()

	c.Add("process-a", "run-1")
	c.Add("process-a", "run-1")
	c.Add("process-b", "run-2")

	require.Equal(t, 2, c.Count("process-a", "run-1"))
	require.Equal(t, 1, c.Count("process-b", "run-2"))
}
