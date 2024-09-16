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
		testCancelTimeout,
		testCompleteTimeout,
		testListTimeout,
	}

	for _, test := range tests {
		test(t, factory)
	}
}

func testCancelTimeout(t *testing.T, factory func() workflow.TimeoutStore) {
	store := factory()
	ctx := context.Background()

	err := store.Create(ctx, "example", "andrew", "1", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "2", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "3", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	timeouts, err := store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 3, len(timeouts))

	for i, timeout := range timeouts {
		require.Equal(t, "example", timeout.WorkflowName)
		require.Equal(t, "andrew", timeout.ForeignID)
		require.Equal(t, fmt.Sprintf("%v", i+1), timeout.RunID)
		require.False(t, timeout.Completed)
		require.WithinDuration(t, time.Now().Add(-time.Hour), timeout.ExpireAt, time.Second)
		require.WithinDuration(t, time.Now(), timeout.CreatedAt, time.Second)
	}

	err = store.Cancel(ctx, 2)
	require.Nil(t, err)

	timeouts, err = store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 2, len(timeouts))
	require.Equal(t, "1", timeouts[0].RunID)
	require.Equal(t, "3", timeouts[1].RunID)
}

func testCompleteTimeout(t *testing.T, factory func() workflow.TimeoutStore) {
	store := factory()
	ctx := context.Background()

	err := store.Create(ctx, "example", "andrew", "1", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "2", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "3", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	timeouts, err := store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 3, len(timeouts))

	for i, timeout := range timeouts {
		require.Equal(t, "example", timeout.WorkflowName)
		require.Equal(t, "andrew", timeout.ForeignID)
		require.Equal(t, fmt.Sprintf("%v", i+1), timeout.RunID)
		require.False(t, timeout.Completed)
		require.WithinDuration(t, time.Now().Add(-time.Hour), timeout.ExpireAt, time.Second)
		require.WithinDuration(t, time.Now(), timeout.CreatedAt, time.Second)
	}

	err = store.Complete(ctx, 2)
	require.Nil(t, err)

	timeouts, err = store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 2, len(timeouts))
	require.Equal(t, "1", timeouts[0].RunID)
	require.Equal(t, "3", timeouts[1].RunID)
}

func testListTimeout(t *testing.T, factory func() workflow.TimeoutStore) {
	store := factory()
	ctx := context.Background()

	err := store.Create(ctx, "example", "andrew", "1", int(statusStarted), time.Now().Add(-time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "2", int(statusMiddle), time.Now().Add(time.Hour))
	require.Nil(t, err)

	err = store.Create(ctx, "example", "andrew", "3", int(statusEnd), time.Now().Add(time.Hour*2))
	require.Nil(t, err)

	timeout, err := store.List(ctx, "example")
	require.Nil(t, err)

	require.Equal(t, 3, len(timeout))
}
