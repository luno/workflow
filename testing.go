package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TriggerCallbackOn[Type any, Status StatusType, Payload any](t testing.TB, w *Workflow[Type, Status], foreignID, runID string, waitFor Status, p Payload) {
	if t == nil {
		panic("TriggerCallbackOn can only be used for testing")
	}

	ctx := context.TODO()

	_, err := w.Await(ctx, foreignID, runID, waitFor)
	require.Nil(t, err)

	b, err := json.Marshal(p)
	require.Nil(t, err)

	err = w.Callback(ctx, foreignID, waitFor, bytes.NewReader(b))
	require.Nil(t, err)
}

func AwaitTimeoutInsert[Type any, Status StatusType](t testing.TB, w *Workflow[Type, Status], foreignID, runID string, waitFor Status) {
	if t == nil {
		panic("AwaitTimeout can only be used for testing")
	}

	var found bool
	for !found {
		if w.ctx.Err() != nil {
			return
		}

		ls, err := w.timeoutStore.List(w.ctx, w.Name)
		require.Nil(t, err)

		for _, l := range ls {
			if l.Status != int(waitFor) {
				continue
			}

			if l.ForeignID != foreignID {
				continue
			}

			if l.RunID != runID {
				continue
			}

			found = true
			break
		}
	}
}

func Require[Type any, Status StatusType](t testing.TB, w *Workflow[Type, Status], foreignID string, waitFor Status, expected Type) {
	if t == nil {
		panic("Require can only be used for testing")
	}

	if !w.statusGraph.IsValid(int(waitFor)) {
		t.Error(fmt.Sprintf(`Status provided is not configured for workflow: "%v" (Workflow: %v)`, waitFor, w.Name))
		return
	}

	testingStore, ok := w.recordStore.(TestingRecordStore)
	if !ok {
		panic("Require function requires TestingRecordStore implementation for record store dependency")
	}

	var runID string
	for runID == "" {
		latest, err := w.recordStore.Latest(context.Background(), w.Name, foreignID)
		if errors.Is(err, ErrRecordNotFound) {
			continue
		} else {
			require.Nil(t, err)
		}

		runID = latest.RunID
	}

	var wr *Record
	for wr == nil {
		offset := testingStore.SnapshotOffset(w.Name, foreignID, runID)
		snapshots := testingStore.Snapshots(w.Name, foreignID, runID)
		for i, r := range snapshots {
			if offset > i {
				continue
			}

			if r.Status == int(waitFor) {
				wr = r
			}

			testingStore.SetSnapshotOffset(w.Name, foreignID, runID, offset+1)
		}
	}

	var actual Type
	err := Unmarshal(wr.Object, &actual)
	require.Nil(t, err)

	// Due to nuances in encoding libraries such as json with the ability to implement custom
	// encodings the marshaling and unmarshalling of an object could result in a different output
	// than the one provided unbeknown to the user. Calling Marshal and Unmarshal on `expected`
	// means that the same operations take place on the type and thus the unmarshaled versions
	// should match.
	encoded, err := Marshal(&expected)
	require.Nil(t, err)

	var normalisedExpected Type
	err = Unmarshal(encoded, &normalisedExpected)
	require.Nil(t, err)

	require.Equal(t, normalisedExpected, actual)
}

// NewTestingRun should be used when testing logic that defines a workflow.Run as a parameter. This is usually the
// case in unit tests and would not normally be found when doing an Acceptance test for the entire workflow.
func NewTestingRun[Type any, Status StatusType](t *testing.T, wr Record, object Type, opts ...TestingRunOption) Run[Type, Status] {
	var options testingRunOpts
	for _, opt := range opts {
		opt(&options)
	}

	return Run[Type, Status]{
		TypedRecord: TypedRecord[Type, Status]{
			Record: wr,
			Status: Status(wr.Status),
			Object: &object,
		},
		controller: &options.controller,
	}
}

type testingRunOpts struct {
	controller testingRunStateController
}

type TestingRunOption func(*testingRunOpts)

func WithPauseFn(pause func(ctx context.Context) error) TestingRunOption {
	return func(opts *testingRunOpts) {
		opts.controller.pause = pause
	}
}

func WithResumeFn(resume func(ctx context.Context) error) TestingRunOption {
	return func(opts *testingRunOpts) {
		opts.controller.resume = resume
	}
}

func WithCancelFn(cancel func(ctx context.Context) error) TestingRunOption {
	return func(opts *testingRunOpts) {
		opts.controller.cancel = cancel
	}
}

func WithDeleteDataFn(deleteData func(ctx context.Context) error) TestingRunOption {
	return func(opts *testingRunOpts) {
		opts.controller.deleteData = deleteData
	}
}

type testingRunStateController struct {
	pause      func(ctx context.Context) error
	cancel     func(ctx context.Context) error
	resume     func(ctx context.Context) error
	deleteData func(ctx context.Context) error
}

func (c *testingRunStateController) Pause(ctx context.Context) error {
	if c.pause == nil {
		return nil
	}

	return c.pause(ctx)
}

func (c *testingRunStateController) Cancel(ctx context.Context) error {
	if c.cancel == nil {
		return nil
	}

	return c.cancel(ctx)
}

func (c *testingRunStateController) Resume(ctx context.Context) error {
	if c.resume == nil {
		return nil
	}

	return c.resume(ctx)
}

func (c *testingRunStateController) DeleteData(ctx context.Context) error {
	if c.deleteData == nil {
		return nil
	}

	return c.deleteData(ctx)
}

var _ RunStateController = (*testingRunStateController)(nil)
