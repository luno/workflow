package adaptertest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func RunTimeoutStoreTest(t *testing.T, factory func() workflow.TimeoutStore) {
	tests := []func(t *testing.T, factory func() workflow.TimeoutStore){
		testCompleteAndCancelTimeout,
		testListTimeout,
	}

	for _, test := range tests {
		test(t, factory)
	}
}

// testCompleteAndCancelTimeout runs a subtest that verifies completing and cancelling
// timeouts in a TimeoutStore.
//
// It obtains a store from the provided factory, seeds it with three timeouts,
// marks the timeout with RunID "2" as completed and the one with RunID "3" as
// cancelled, then calls ListValid for the "example" workflow with statusStarted
// and the current time. The test asserts that exactly one valid timeout remains
// and that its fields match the expected values.
func testCompleteAndCancelTimeout(t *testing.T, factory func() workflow.TimeoutStore) {
	t.Run("Complete and Cancel timeouts", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		seed(t, store, 3)

		err := store.Complete(ctx, 2)
		require.NoError(t, err)

		err = store.Cancel(ctx, 3)
		require.NoError(t, err)

		timeouts, err := store.ListValid(ctx, "example", int(statusStarted), time.Now())
		require.NoError(t, err)

		expect(t, 1, timeouts)
	})
}

// seed seeds the provided TimeoutStore with `count` expired timeouts for use in tests.
// Each created timeout uses workflow name "example", foreign ID "andrew", a RunID of
// "1".."count", status `statusStarted` and an ExpireAt timestamp one hour in the past.
// The helper uses a background context and fails the test if any Create call returns an error.
func seed(t *testing.T, store workflow.TimeoutStore, count int) {
	ctx := context.Background()
	for i := range count {
		err := store.Create(
			ctx,
			"example",
			"andrew",
			fmt.Sprintf("%v", i+1),
			int(statusStarted),
			time.Now().Add(-time.Hour),
		)
		require.NoError(t, err)
	}
}

func expect(t *testing.T, count int, actual []workflow.TimeoutRecord) {
	// Assert the length
	require.Equal(t, count, len(actual))

	// Validate the contents
	for i, timeout := range actual {
		require.Equal(t, "example", timeout.WorkflowName)
		require.Equal(t, "andrew", timeout.ForeignID)
		require.Equal(t, fmt.Sprintf("%v", i+1), timeout.RunID)
		require.False(t, timeout.Completed)
		require.WithinDuration(t, time.Now().Add(-time.Hour), timeout.ExpireAt, allowedTimeDeviation)
		require.WithinDuration(t, time.Now(), timeout.CreatedAt, allowedTimeDeviation)
	}
}

// testListTimeout runs a subtest that verifies a TimeoutStore's List method returns all seeded timeouts.
// 
// It obtains a store from the provided factory, seeds it with three expired timeouts, calls List for the
// "example" workflow and asserts the call succeeds and the returned slice matches the seeded entries.
func testListTimeout(t *testing.T, factory func() workflow.TimeoutStore) {
	t.Run("List timeouts", func(t *testing.T) {
		store := factory()
		ctx := context.Background()

		seed(t, store, 3)

		timeouts, err := store.List(ctx, "example")
		require.NoError(t, err)

		expect(t, 3, timeouts)
	})
}
