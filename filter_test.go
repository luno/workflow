package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestMakeFilter(t *testing.T) {
	t.Run("Filter by RunState", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByRunState(workflow.RunStateInitiated))
		require.True(t, filter.ByRunState().Enabled, "Expected RunState filter to be enabled")
		require.Equal(t, "1", filter.ByRunState().Value, "Expected RunState filter value to be 5")
	})

	t.Run("Filter by ForeignID", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByForeignID("test value"))
		require.True(t, filter.ByForeignID().Enabled, "Expected foreign ID filter to be enabled")
		require.Equal(t, "test value", filter.ByForeignID().Value, "Expected foreign ID filter value to be 5")
	})

	t.Run("Filter by Status", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9))
		require.True(t, filter.ByStatus().Enabled, "Expected status filter to be enabled")
		require.Equal(t, "9", filter.ByStatus().Value, "Expected status filter value to be 5")
	})
}
