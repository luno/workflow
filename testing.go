package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TriggerCallbackOn waits for the record with the given foreignID to reach waitForStatus
// and then invokes the workflow's Callback with p marshalled as JSON.
//
// It panics if t is nil or if api is not a *Workflow[Type, Status]. The function blocks
// until a matching Record is observed; if JSON marshalling of p or the Callback call
// returns an error the test is failed via require.NoError.
func TriggerCallbackOn[Type any, Status StatusType, Payload any](
	t testing.TB,
	api API[Type, Status],
	foreignID, runID string,
	waitForStatus Status,
	p Payload,
) {
	if t == nil {
		panic("TriggerCallbackOn can only be used for testing")
	}

	w, ok := api.(*Workflow[Type, Status])
	if !ok {
		panic("*workflow.Workflow required for testing utility functions")
	}

	_ = waitFor(t, w, foreignID, func(r *Record) (bool, error) {
		return r.Status == int(waitForStatus), nil
	})

	b, err := json.Marshal(p)
	require.NoError(t, err)

	err = w.Callback(w.ctx, foreignID, waitForStatus, bytes.NewReader(b))
	require.NoError(t, err)
}

// AwaitTimeoutInsert waits until a timeout entry with the given foreignID, runID and
// status appears in the workflow's timeout store or the workflow context is cancelled.
// It panics if t is nil or if api is not a *Workflow[Type, Status].
//
// The function repeatedly lists the workflow's timeout entries and returns once a
// matching entry (Status == waitFor, ForeignID == foreignID, RunID == runID) is found.
func AwaitTimeoutInsert[Type any, Status StatusType](
	t testing.TB,
	api API[Type, Status],
	foreignID, runID string,
	waitFor Status,
) {
	if t == nil {
		panic("AwaitTimeoutInsert can only be used for testing")
	}

	w, ok := api.(*Workflow[Type, Status])
	if !ok {
		panic("*workflow.Workflow required for testing utility functions")
	}

	var found bool
	for !found {
		if w.ctx.Err() != nil {
			return
		}

		ls, err := w.timeoutStore.List(w.ctx, w.Name())
		require.NoError(t, err)
		if len(ls) == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

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

// The call blocks until a matching record is observed or the test context ends.
func Require[Type any, Status StatusType](
	t testing.TB,
	api API[Type, Status],
	foreignID string,
	waitForStatus Status,
	expected Type,
) {
	if t == nil {
		panic("Require can only be used for testing")
	}

	w, ok := api.(*Workflow[Type, Status])
	if !ok {
		panic("*workflow.Workflow required for testing utility functions")
	}

	if !w.statusGraph.IsValid(int(waitForStatus)) {
		t.Error(
			fmt.Sprintf(`Status provided is not configured for workflow: "%v" (Workflow: %v)`, waitForStatus, w.Name()),
		)
		return
	}

	wr := waitFor(t, w, foreignID, func(r *Record) (bool, error) {
		return r.Status == int(waitForStatus), nil
	})

	var actual Type
	err := Unmarshal(wr.Object, &actual)
	require.NoError(t, err)

	// Due to nuances in encoding libraries such as json with the ability to implement custom
	// encodings the marshaling and unmarshalling of an object could result in a different output
	// than the one provided unbeknown to the user. Calling Marshal and Unmarshal on `expected`
	// means that the same operations take place on the type and thus the unmarshaled versions
	// should match.
	encoded, err := Marshal(&expected)
	require.NoError(t, err)

	var normalisedExpected Type
	err = Unmarshal(encoded, &normalisedExpected)
	require.NoError(t, err)

	require.Equal(t, normalisedExpected, actual)
}

// WaitFor waits until a Run for the given foreignID satisfies the supplied predicate.
// 
// It requires a testing.TB and an API that is a *Workflow; it panics if t is nil or
// api is not the expected workflow type. The predicate fn is invoked with successive
// Run snapshots (converted from stored Records); when fn returns true the wait stops.
// If fn returns a non-nil error the wait aborts and the error fails the test.
//
// The conversion from Record to Run uses buildRun; any conversion error is asserted
// with require.NoError and will fail the test. This function does not return a value.
func WaitFor[Type any, Status StatusType](
	t testing.TB,
	api API[Type, Status],
	foreignID string,
	fn func(r *Run[Type, Status]) (bool, error),
) {
	if t == nil {
		panic("WaitFor can only be used for testing")
	}

	w, ok := api.(*Workflow[Type, Status])
	if !ok {
		panic("*workflow.Workflow required for testing utility functions")
	}

	waitFor(t, w, foreignID, func(r *Record) (bool, error) {
		run, err := buildRun[Type, Status](w.recordStore.Store, r)
		require.NoError(t, err)

		return fn(run)
	})
}

// waitFor polls the workflow's records for the given foreignID until a predicate
// applied to a snapshot returns true, then returns that matching Record.
//
// The function requires the workflow's recordStore to implement TestingRecordStore;
// it panics if that is not the case. It first waits until a latest Record exists
// for the foreignID (using Latest) to obtain the current run ID, then iterates
// the snapshots for that run calling fn on each snapshot. When fn returns true
// (and no error), the corresponding snapshot is returned. Any errors produced by
// fn are propagated as test failures.
//
// This function blocks until a matching snapshot is found and therefore may hang
// indefinitely if no snapshot satisfies the predicate. It returns a non-nil
// *Record when a match is found. Parameters are:
//  - t: the testing.TB used for assertions.
//  - w: the Workflow instance under test.
//  - foreignID: the ID of the external entity whose records are inspected.
//  - fn: predicate applied to each snapshot; should return (true, nil) to select
//    a snapshot.
func waitFor[Type any, Status StatusType](
	t testing.TB,
	w *Workflow[Type, Status],
	foreignID string,
	fn func(r *Record) (bool, error),
) *Record {
	testingStore, ok := w.recordStore.(TestingRecordStore)
	if !ok {
		panic("TestingRecordStore implementation for record store dependency required")
	}

	var runID string
	for runID == "" {
		latest, err := w.recordStore.Latest(context.Background(), w.Name(), foreignID)
		if errors.Is(err, ErrRecordNotFound) {
			continue
		} else {
			require.NoError(t, err)
		}

		runID = latest.RunID
	}

	// Reset the offset run through all the changes and not just from the offset
	// testingStore.SetSnapshotOffset(w.name, foreignID, runID, 0)

	var wr Record
	for wr.RunID == "" {
		snapshots := testingStore.Snapshots(w.Name(), foreignID, runID)
		for _, r := range snapshots {
			ok, err := fn(r)
			require.NoError(t, err)

			if ok {
				wr = *r
			}
		}
	}

	return &wr
}

// NewTestingRun should be used when testing logic that defines a workflow.Run as a parameter. This is usually the
// case in unit tests and would not normally be found when doing an Acceptance test for the entire workflow.
func NewTestingRun[Type any, Status StatusType](
	t *testing.T,
	wr Record,
	object Type,
	opts ...TestingRunOption,
) *Run[Type, Status] {
	if t == nil {
		panic("Cannot use NewTestingRun without *testing.T parameter")
	}

	var options testingRunOpts
	for _, opt := range opts {
		opt(&options)
	}

	return &Run[Type, Status]{
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

func (c *testingRunStateController) Pause(ctx context.Context, reason string) error {
	if c.pause == nil {
		return nil
	}

	return c.pause(ctx)
}

func (c *testingRunStateController) Cancel(ctx context.Context, reason string) error {
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

func (c *testingRunStateController) DeleteData(ctx context.Context, reason string) error {
	if c.deleteData == nil {
		return nil
	}

	return c.deleteData(ctx)
}

var _ RunStateController = (*testingRunStateController)(nil)
