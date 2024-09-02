package adaptertest

import (
	"context"
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

	timeout, err := store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 3, len(timeout))

	require.Equal(t, "example", timeout[0].WorkflowName)
	require.Equal(t, "andrew", timeout[0].ForeignID)
	require.Equal(t, "1", timeout[0].RunID)
	require.False(t, timeout[0].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[0].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[0].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[1].WorkflowName)
	require.Equal(t, "andrew", timeout[1].ForeignID)
	require.Equal(t, "2", timeout[1].RunID)
	require.False(t, timeout[1].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[1].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[1].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[2].WorkflowName)
	require.Equal(t, "andrew", timeout[2].ForeignID)
	require.Equal(t, "3", timeout[2].RunID)
	require.False(t, timeout[2].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[2].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[2].CreatedAt, time.Second)

	err = store.Cancel(ctx, 2)
	require.Nil(t, err)

	timeout, err = store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 2, len(timeout))

	require.Equal(t, "example", timeout[0].WorkflowName)
	require.Equal(t, "andrew", timeout[0].ForeignID)
	require.Equal(t, "1", timeout[0].RunID)
	require.False(t, timeout[0].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[0].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[0].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[1].WorkflowName)
	require.Equal(t, "andrew", timeout[1].ForeignID)
	require.Equal(t, "3", timeout[1].RunID)
	require.False(t, timeout[1].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[1].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[1].CreatedAt, time.Second)
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

	timeout, err := store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 3, len(timeout))

	require.Equal(t, "example", timeout[0].WorkflowName)
	require.Equal(t, "andrew", timeout[0].ForeignID)
	require.Equal(t, "1", timeout[0].RunID)
	require.False(t, timeout[0].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[0].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[0].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[1].WorkflowName)
	require.Equal(t, "andrew", timeout[1].ForeignID)
	require.Equal(t, "2", timeout[1].RunID)
	require.False(t, timeout[1].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[1].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[1].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[2].WorkflowName)
	require.Equal(t, "andrew", timeout[2].ForeignID)
	require.Equal(t, "3", timeout[2].RunID)
	require.False(t, timeout[2].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[2].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[2].CreatedAt, time.Second)

	err = store.Complete(ctx, 2)
	require.Nil(t, err)

	timeout, err = store.ListValid(ctx, "example", int(statusStarted), time.Now())
	require.Nil(t, err)

	require.Equal(t, 2, len(timeout))

	require.Equal(t, "example", timeout[0].WorkflowName)
	require.Equal(t, "andrew", timeout[0].ForeignID)
	require.Equal(t, "1", timeout[0].RunID)
	require.False(t, timeout[0].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[0].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[0].CreatedAt, time.Second)

	require.Equal(t, "example", timeout[1].WorkflowName)
	require.Equal(t, "andrew", timeout[1].ForeignID)
	require.Equal(t, "3", timeout[1].RunID)
	require.False(t, timeout[1].Completed)
	require.WithinDuration(t, time.Now().Add(-time.Hour), timeout[1].ExpireAt, time.Second)
	require.WithinDuration(t, time.Now(), timeout[1].CreatedAt, time.Second)
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
