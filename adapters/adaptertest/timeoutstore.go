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
