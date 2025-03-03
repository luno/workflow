package workflow_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestWorkflow_OnPauseHook(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	wf := setupHookTest(t, func(b *workflow.Builder[MyType, status]) {
		b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[MyType, status]) (status, error) {
			return r.Pause(ctx)
		}, StatusMiddle)

		b.OnPause(func(ctx context.Context, record *workflow.TypedRecord[MyType, status]) error {
			wg.Done()
			return nil
		})
	})

	foreignID := "andrew"
	_, err := wf.Trigger(context.Background(), foreignID)
	require.Nil(t, err)

	wg.Wait()
}

func TestWorkflow_OnCancelHook(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	wf := setupHookTest(t, func(b *workflow.Builder[MyType, status]) {
		b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[MyType, status]) (status, error) {
			return r.Cancel(ctx)
		}, StatusMiddle)

		b.OnCancel(func(ctx context.Context, record *workflow.TypedRecord[MyType, status]) error {
			wg.Done()
			return nil
		})
	})

	foreignID := "andrew"
	_, err := wf.Trigger(context.Background(), foreignID)
	require.Nil(t, err)

	wg.Wait()
}

func TestWorkflow_OnCompleteHook(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	wf := setupHookTest(t, func(b *workflow.Builder[MyType, status]) {
		b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[MyType, status]) (status, error) {
			return StatusEnd, nil
		}, StatusEnd)

		b.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[MyType, status]) error {
			wg.Done()
			return nil
		})
	})

	foreignID := "andrew"
	_, err := wf.Trigger(context.Background(), foreignID)
	require.Nil(t, err)

	wg.Wait()
}

func setupHookTest(t *testing.T, custom func(b *workflow.Builder[MyType, status])) *workflow.Workflow[MyType, status] {
	b := workflow.NewBuilder[MyType, status]("hooks")

	custom(b)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	return wf
}
