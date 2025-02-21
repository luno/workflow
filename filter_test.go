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
		require.Equal(t, "1", filter.ByRunState().Value(), "Expected RunState filter value to be 5")
	})

	t.Run("Filter by RunState - Multi", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByRunState(workflow.RunStateInitiated, workflow.RunStateRunning))
		require.True(t, filter.ByRunState().Enabled, "Expected RunState filter to be enabled")
		require.Equal(t, []string{"1", "2"}, filter.ByRunState().MultiValues(), "Expected RunState filter value to be {'1', '2'}")
	})

	t.Run("Filter by ForeignID", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByForeignID("test value"))
		require.True(t, filter.ByForeignID().Enabled, "Expected foreign ID filter to be enabled")
		require.Equal(t, "test value", filter.ByForeignID().Value(), "Expected foreign ID filter value to be 5")
	})

	t.Run("Filter by ForeignID - Multi", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByForeignID("first value", "second value"))
		require.True(t, filter.ByForeignID().Enabled, "Expected foreign ID filter to be enabled")
		require.Equal(t, []string{"first value", "second value"}, filter.ByForeignID().MultiValues(), "Expected foreign ID filter value to be 'first value' and 'second value'")
	})

	t.Run("Filter by Status", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9))
		require.True(t, filter.ByStatus().Enabled, "Expected status filter to be enabled")
		require.Equal(t, "9", filter.ByStatus().Value(), "Expected status filter value to be 5")
	})

	t.Run("Filter by Status - Multi", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9, 12))
		require.True(t, filter.ByStatus().Enabled, "Expected status filter to be enabled")
		require.Equal(t, []string{"9", "12"}, filter.ByStatus().MultiValues(), "Expected status filter value to be {'9', '12'}")
	})

	t.Run("Matches", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9))
		require.True(t, filter.ByStatus().Matches("9"))
	})

	t.Run("Matches - Multi", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9, 12))
		require.True(t, filter.ByStatus().Matches("9"))
		require.True(t, filter.ByStatus().Matches("12"))
	})
}
