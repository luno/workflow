package workflow_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memstreamer"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
)

func TestRequireForCircularStatus(t *testing.T) {
	type Counter struct {
		Count int
	}

	b := workflow.NewBuilder[Counter, status]("circular flow")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[Counter, status]) (bool, error) {
		r.Object.Count += 1
		return true, nil
	}, StatusMiddle)
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[Counter, status]) (bool, error) {
		return r.Object.Count < 2, nil
	}, StatusStart)
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Record[Counter, status]) (bool, error) {
		r.Object.Count += 1
		return r.Object.Count > 2, nil
	}, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memtimeoutstore.New(),
		memrolescheduler.New(),
	)

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid, StatusStart)
	jtest.RequireNil(t, err)

	workflow.Require(t, wf, fid, StatusStart, Counter{Count: 0})
	workflow.Require(t, wf, fid, StatusMiddle, Counter{Count: 1})
	workflow.Require(t, wf, fid, StatusStart, Counter{Count: 1})
	workflow.Require(t, wf, fid, StatusMiddle, Counter{Count: 2})
	workflow.Require(t, wf, fid, StatusEnd, Counter{Count: 3})
}
