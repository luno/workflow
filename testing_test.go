package workflow_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestTriggerCallbackOn_validation(t *testing.T) {
	t.Run("TriggerCallbackOn must be used in tests", func(t *testing.T) {
		require.PanicsWithValue(t,
			"TriggerCallbackOn can only be used for testing",
			func() {
				workflow.TriggerCallbackOn[string, status](nil, nil, "", "", StatusStart, "")
			}, "Not providing a testing.T or testing.B should panic")
	})

	t.Run("TriggerCallbackOn must be provided with a workflow.Workflow", func(t *testing.T) {
		require.PanicsWithValue(t,
			"*workflow.Workflow required for testing utility functions",
			func() {
				api := &apiImpl[string, status]{}
				workflow.Require(t, api, "", StatusEnd, "")
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})

	t.Run("TriggerCallbackOn must be provided with a workflow that is using a record store that implements TestingRecordStore", func(t *testing.T) {
		require.PanicsWithValue(t,
			"TestingRecordStore implementation for record store dependency required",
			func() {
				b := workflow.NewBuilder[string, status]("test")
				b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
					return 0, nil
				}, StatusEnd)

				w := b.Build(
					memstreamer.New(),
					nil,
					memrolescheduler.New(),
				)

				workflow.TriggerCallbackOn(t, w, "", "", StatusEnd, "")
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})
}

func TestAwaitTimeoutInsert_validation(t *testing.T) {
	t.Run("AwaitTimeoutInsert must be used in tests", func(t *testing.T) {
		require.PanicsWithValue(t,
			"AwaitTimeoutInsert can only be used for testing",
			func() {
				workflow.AwaitTimeoutInsert[string, status](nil, nil, "", "", StatusStart)
			}, "Not providing a testing.T or testing.B should panic")
	})

	t.Run("AwaitTimeoutInsert must be provided with a workflow that is using a record store that implements TestingRecordStore", func(t *testing.T) {
		require.PanicsWithValue(t,
			"TestingRecordStore implementation for record store dependency required",
			func() {
				b := workflow.NewBuilder[string, status]("test")
				b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
					return 0, nil
				}, StatusEnd)

				wf := b.Build(
					memstreamer.New(),
					nil,
					memrolescheduler.New(),
				)

				workflow.Require(t, wf, "", StatusEnd, "")
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})

	t.Run("AwaitTimeoutInsert must be provided with a *workflow.Workflow", func(t *testing.T) {
		require.PanicsWithValue(t,
			"*workflow.Workflow required for testing utility functions",
			func() {
				api := &apiImpl[string, status]{}
				workflow.AwaitTimeoutInsert(t, api, "", "", StatusEnd)
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})
}

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

	ctx := t.Context()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid)
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
			"TestingRecordStore implementation for record store dependency required",
			func() {
				b := workflow.NewBuilder[string, status]("test")
				b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
					return 0, nil
				}, StatusEnd)

				wf := b.Build(
					memstreamer.New(),
					nil,
					memrolescheduler.New(),
				)

				workflow.Require(t, wf, "", StatusEnd, "")
			}, "Not providing a workflow using a TestingRecordStore implemented record store should panic")
	})

	t.Run("Require must be provided with a *workflow.Workflow", func(t *testing.T) {
		require.PanicsWithValue(t,
			"*workflow.Workflow required for testing utility functions",
			func() {
				api := &apiImpl[string, status]{}
				workflow.Require(t, api, "", StatusEnd, "")
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

func TestWaitFor(t *testing.T) {
	b := workflow.NewBuilder[string, status]("test")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid)
	require.Nil(t, err)

	workflow.WaitFor(t, wf, fid, func(r *workflow.Run[string, status]) (bool, error) {
		return r.RunState == workflow.RunStateCompleted, nil
	})
}

var _ workflow.API[string, status] = (*apiImpl[string, status])(nil)

type apiImpl[Type any, Status workflow.StatusType] struct{}

func (a apiImpl[Type, Status]) Name() string {
	return "test"
}

func (a apiImpl[Type, Status]) Trigger(ctx context.Context, foreignID string, opts ...workflow.TriggerOption[Type, Status]) (runID string, err error) {
	return "", nil
}

func (a apiImpl[Type, Status]) Schedule(foreignID string, spec string, opts ...workflow.ScheduleOption[Type, Status]) error {
	return nil
}

func (a apiImpl[Type, Status]) Await(ctx context.Context, foreignID, runID string, status Status, opts ...workflow.AwaitOption) (*workflow.Run[Type, Status], error) {
	return &workflow.Run[Type, Status]{}, nil
}

func (a apiImpl[Type, Status]) Callback(ctx context.Context, foreignID string, status Status, payload io.Reader) error {
	return nil
}

func (a apiImpl[Type, Status]) Run(ctx context.Context) {}

func (a apiImpl[Type, Status]) Stop() {}
