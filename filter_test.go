package workflow_test

import (
	"testing"
	"time"

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

	t.Run("Filter by Created At After", func(t *testing.T) {
		d := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
		filter := workflow.MakeFilter(workflow.FilterByCreatedAtAfter(d))
		require.True(t, filter.ByCreatedAtAfter().Enabled, "Expected created at after filter to be enabled")
		require.Equal(t, d, filter.ByCreatedAtAfter().Value(), "Expected created at after filter value to be 1st of august 2025")
	})

	t.Run("Filter by Created At Before", func(t *testing.T) {
		d := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
		filter := workflow.MakeFilter(workflow.FilterByCreatedAtBefore(d))
		require.True(t, filter.ByCreatedAtBefore().Enabled, "Expected created at before filter to be enabled")
		require.Equal(t, d, filter.ByCreatedAtBefore().Value(), "Expected created at before filter value to be 1st of august 2025")
	})

	t.Run("Matches", func(t *testing.T) {
		t0 := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
		t1 := time.Date(2025, time.August, 10, 0, 0, 0, 0, time.UTC)

		filter := workflow.MakeFilter(workflow.FilterByStatus(9),
			workflow.FilterByCreatedAtAfter(t0),
			workflow.FilterByCreatedAtBefore(t1))
		require.True(t, filter.ByStatus().Matches("9"))

		match := time.Date(2025, time.August, 5, 0, 0, 0, 0, time.UTC)
		require.True(t, filter.ByCreatedAtAfter().Matches(match))
		require.True(t, filter.ByCreatedAtBefore().Matches(match))
	})

	t.Run("Matches - Multi", func(t *testing.T) {
		filter := workflow.MakeFilter(workflow.FilterByStatus(9, 12))
		require.True(t, filter.ByStatus().Matches("9"))
		require.True(t, filter.ByStatus().Matches("12"))
	})
}
