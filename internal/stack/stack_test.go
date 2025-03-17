package stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/stack"
)

func TestTrace(t *testing.T) {
	t.Run("Basic 0 depth", func(t *testing.T) {
		trace := stack.Trace(0)
		require.Contains(t, trace, "stack_test.go")
		require.Contains(t, trace, "/internal/stack/stack_test.go")
	})

	t.Run("Exclude internal 3 deep stack", func(t *testing.T) {
		var trace string
		grandchild := func() {
			trace = stack.Trace(3)
		}
		child := func() {
			grandchild()
		}
		parent := func() {
			child()
		}
		external := func() {
			parent()
		}

		external()
		t.Log(trace)
	})
}
