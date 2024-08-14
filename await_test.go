package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestAwait(t *testing.T) {
	b := workflow.NewBuilder[string, status]("consumer lag")
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
			*r.Object = "hello world"
			return StatusEnd, nil
		},
		StatusEnd,
	)
	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	runID, err := wf.Trigger(ctx, "1", StatusStart)
	jtest.RequireNil(t, err)

	res, err := wf.Await(ctx, "1", runID, StatusEnd, workflow.WithAwaitPollingFrequency(10*time.Nanosecond))
	jtest.RequireNil(t, err)

	require.Equal(t, StatusEnd, res.Status)
	require.Equal(t, "hello world", *res.Object)
}
