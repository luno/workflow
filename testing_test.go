package workflow_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestRequireForCircularStatus(t *testing.T) {
	type Counter struct {
		Count int
	}

	b := workflow.NewBuilder[Counter, status]("circular flow")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[Counter, status]) (status, error) {
		r.Object.Count += 1
		return StatusMiddle, nil
	}, StatusMiddle)
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Run[Counter, status]) (status, error) {
		if r.Object.Count < 2 {
			return StatusStart, nil
		}

		r.Object.Count += 1
		return StatusEnd, nil
	}, StatusStart, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid, StatusStart)
	require.Nil(t, err)

	workflow.Require(t, wf, fid, StatusStart, Counter{Count: 0})
	workflow.Require(t, wf, fid, StatusMiddle, Counter{Count: 1})
	workflow.Require(t, wf, fid, StatusStart, Counter{Count: 1})
	workflow.Require(t, wf, fid, StatusMiddle, Counter{Count: 2})
	workflow.Require(t, wf, fid, StatusEnd, Counter{Count: 3})
}
