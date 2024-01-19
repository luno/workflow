package adaptertest

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestRoleScheduler(t *testing.T, factory func() workflow.RoleScheduler) {
	tests := []func(t *testing.T, rs workflow.RoleScheduler){
		testReturnedContext,
		testLocking,
		testReleasing,
	}

	for _, test := range tests {
		storeForTesting := factory()
		test(t, storeForTesting)
	}
}

func testReturnedContext(t *testing.T, rs workflow.RoleScheduler) {
	t.Run("Ensure that the passed in context is a parent of the returned context", func(t *testing.T) {
		ctx := context.Background()
		ctxWithValue := context.WithValue(ctx, "parent", "context")

		ctx2, cancel, err := rs.Await(ctxWithValue, "leader")
		jtest.RequireNil(t, err)

		t.Cleanup(cancel)

		require.Equal(t, "context", ctx2.Value("parent"))
	})
}

func testLocking(t *testing.T, rs workflow.RoleScheduler) {
	t.Run("Ensure role is locked and successive calls are blocked", func(t *testing.T) {
		ctx := context.Background()
		ctxWithValue := context.WithValue(ctx, "parent", "context")

		ctx2, cancel, err := rs.Await(ctxWithValue, "leader")
		jtest.RequireNil(t, err)

		t.Cleanup(cancel)

		roleReleased := make(chan bool, 1)
		go func(done chan bool) {
			_, _, err := rs.Await(ctxWithValue, "leader")
			jtest.RequireNil(t, err)

			roleReleased <- true

			// Ensure that the passed in context is a parent of the returned context
			require.Equal(t, "context", ctx2.Value("parent"))
		}(roleReleased)

		timeout := time.NewTicker(1 * time.Second).C
		select {
		case <-timeout:
			// Pass -  timeout expected to return first as the role has not been released
			return
		case <-roleReleased:
			t.Fail()
			return
		}
	})
}

func testReleasing(t *testing.T, rs workflow.RoleScheduler) {
	t.Run("Ensure role is released on context cancellation", func(t *testing.T) {
		ctx := context.Background()

		_, cancel, err := rs.Await(ctx, "leader")
		jtest.RequireNil(t, err)

		t.Cleanup(cancel)

		ctx2, cancel2 := context.WithCancel(context.Background())
		t.Cleanup(cancel2)

		roleReleased := make(chan bool, 1)
		go func(ctx context.Context, done chan bool) {
			_, _, err := rs.Await(ctx2, "leader")
			jtest.RequireNil(t, err)

			roleReleased <- true
		}(ctx2, roleReleased)

		cancel()

		timeout := time.NewTicker(3 * time.Second).C
		select {
		case <-timeout:
			t.Fail()
			return
		case <-roleReleased:
			// Pass - context passed into the first attempt at gaining the role is cancelled and should
			// result in a releasing of the role.
			return
		}
	})
}
