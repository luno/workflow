package workflow_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestRequire(t *testing.T) {
	b := workflow.NewBuilder[testCustomMarshaler, status]("test")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[testCustomMarshaler, status]) (status, error) {
		*r.Object = "Lower"
		return StatusEnd, nil
	}, StatusEnd)

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

	workflow.Require(t, wf, fid, StatusEnd, "Lower")
}

func TestRequire_validation(t *testing.T) {
	t.Run("Require must be provided with testing and will panic without", func(t *testing.T) {
		require.PanicsWithValue(t,
			"Require can only be used for testing",
			func() {
				workflow.Require[string, status](nil, nil, "", StatusEnd, "")
			}, "Not providing a testing.T or testing.B should panic")
	})

	t.Run("Require must be provided with a workflow that is using a record store that implements TestingRecordStore", func(t *testing.T) {
		require.PanicsWithValue(t,
			"Require function requires TestingRecordStore implementation for record store dependency",
			func() {
				b := workflow.NewBuilder[string, status]("test")
				b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
					return r.Cancel(ctx)
				}, StatusEnd)

				wf := b.Build(
					memstreamer.New(),
					nil,
					memrolescheduler.New(),
				)

				workflow.Require(t, wf, "", StatusEnd, "")
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})
}

// testCustomMarshaler is for testing and implements custom and weird behaviour via the
// MarshalJSON method and this is to test that the Require function can successfully compare
// the actual and expected by running them through the same encoding / decoding process.
type testCustomMarshaler string

func (t testCustomMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToLower(string(t)))
}

func TestWaitFor_validation(t *testing.T) {
	require.PanicsWithValue(t,
		"WaitFor can only be used for testing",
		func() {
			workflow.WaitFor[string, status](nil, nil, "", nil)
		}, "Not providing a testing.T or testing.B should panic")
}

func TestWaitFor(t *testing.T) {
	b := workflow.NewBuilder[string, status]("test")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return r.Cancel(ctx)
	}, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid, StatusStart)
	require.Nil(t, err)

	workflow.WaitFor(t, wf, fid, func(r *workflow.Run[string, status]) (bool, error) {
		return r.RunState == workflow.RunStateCompleted, nil
	})
}
