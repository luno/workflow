package adaptertest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func RunRoleSchedulerTest(t *testing.T, factory func(t *testing.T, instances int) []workflow.RoleScheduler) {
	tests := []func(t *testing.T, factory func(t *testing.T, instances int) []workflow.RoleScheduler){
		testReturnedContext,
		testLocking,
		testReleasing,
	}

	for _, test := range tests {
		test(t, factory)
	}
}

func testReturnedContext(t *testing.T, factory func(t *testing.T, instances int) []workflow.RoleScheduler) {
	t.Run("Ensure that the passed in context is a parent of the returned context", func(t *testing.T) {
		rs := factory(t, 1)
		ctx := context.Background()
		ctxWithValue := context.WithValue(ctx, "parent", "context")

		ctx2, cancel, err := rs[0].Await(ctxWithValue, "leader-cancelled-ctx")
		require.NoError(t, err)

		cancel()

		require.Equal(t, "context", ctx2.Value("parent"))
	})
}

func testLocking(t *testing.T, factory func(t *testing.T, instances int) []workflow.RoleScheduler) {
	t.Run("Ensure role is locked and successive calls are blocked", func(t *testing.T) {
		rs := factory(t, 5)
		ctx := context.Background()
		ctxWithValue := context.WithValue(ctx, "parent", "context")

		rolesObtained := make(chan bool, len(rs))
		for _, rinkInstance := range rs {
			go func(rolesObtained chan bool) {
				_, _, err := rinkInstance.Await(ctxWithValue, "leader-lock")
				require.NoError(t, err)
				rolesObtained <- true
			}(rolesObtained)
		}

		checkInterval := time.NewTicker(50 * time.Millisecond).C
		timeout := time.NewTicker(250 * time.Millisecond).C
		for {
			select {
			case <-timeout:
				// Pass -  timeout expected to return first as the role has not been released
				return
			case <-checkInterval:
				if len(rolesObtained) > 1 {
					require.FailNow(t, "more than one instance received a role lock")
				}
			}
		}
	})
}

func testReleasing(t *testing.T, factory func(t *testing.T, instances int) []workflow.RoleScheduler) {
	t.Run("Ensure role is released on context cancellation", func(t *testing.T) {
		instanceCount := 2
		rs := factory(t, instanceCount)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		passed := make(chan bool)
		go func() {
			_, _, err := rs[0].Await(ctx, "leader-releasing")
			require.NoError(t, err)

			ctx2, cancel2 := context.WithCancel(context.Background())
			go func() {
				_, _, err := rs[1].Await(ctx2, "leader-releasing")
				require.ErrorIs(t, err, context.Canceled)

				// Record that the execution got here.
				passed <- true
			}()

			// Cancel the other caller to test that it unlocks on context cancellation
			cancel2()
		}()

		timeout := time.NewTicker(5 * time.Second).C
		for {
			select {
			case <-timeout:
				require.FailNow(t, "not all instances obtained the lock")
				return
			case <-passed:
				// Expected call stack executed
				return
			}
		}
	})
}
