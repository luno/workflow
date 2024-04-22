package workflow_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

func TestRunState(t *testing.T) {
	testCases := []struct {
		name      string
		workflow  func(recordStore workflow.RecordStore) *workflow.Workflow[string, status]
		inspectAt status
		expected  []workflow.RunState
	}{
		{
			name: "Successful complete path",
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
				return StatusEnd, nil
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStateRunning,
				workflow.RunStateCompleted,
			},
		},
		{
			name: "Paused",
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
				return r.Pause(ctx)
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStateRunning,
				workflow.RunStatePaused,
			},
		},
		{
			name: "Cancelled",
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
				return r.Cancel(ctx)
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStateRunning,
				workflow.RunStateCancelled,
			},
		},
	}

	for _, tc := range testCases {
		fn := tc.workflow
		expected := tc.expected

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recordStore := memrecordstore.New()
			w := fn(recordStore)

			ctx := context.Background()
			w.Run(ctx)
			t.Cleanup(w.Stop)

			// Trigger workflow before it's running to assert that the initial state is workflow.RunStateInitiated
			runID, err := w.Trigger(ctx, "fid", StatusStart)
			jtest.RequireNil(t, err)

			time.Sleep(time.Second)

			snapshots := recordStore.Snapshots(w.Name, "fid", runID)

			for i, state := range expected {
				require.Equal(t, state, snapshots[i].RunState)
			}
		})
	}
}

func buildWorkflow(fn workflow.ConsumerFunc[string, status]) func(recordStore workflow.RecordStore) *workflow.Workflow[string, status] {
	return func(recordStore workflow.RecordStore) *workflow.Workflow[string, status] {
		b := workflow.NewBuilder[string, status]("example")
		b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
			return StatusMiddle, nil
		}, StatusMiddle).WithOptions(
			workflow.PollingFrequency(time.Millisecond * 10),
		)

		b.AddStep(StatusMiddle,
			fn,
			StatusEnd,
		).WithOptions(
			workflow.PollingFrequency(time.Millisecond * 10),
		)

		w := b.Build(
			memstreamer.New(),
			recordStore,
			memtimeoutstore.New(),
			memrolescheduler.New(),
			workflow.WithDebugMode(),
			workflow.WithOutboxConfig(workflow.WithOutboxPollingFrequency(time.Millisecond*5)),
		)

		return w
	}
}

func TestWorkflowRunStateController(t *testing.T) {
	mu := sync.Mutex{}
	canRelease := false

	type myObject struct {
		Name string
		Car  string
	}

	b := workflow.NewBuilder[myObject, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[myObject, status]) (status, error) {
		// This consumer should block until it's released
		for {
			mu.Lock()
			if !canRelease {
				mu.Unlock()
				time.Sleep(time.Millisecond * 10)
				continue
			}
			mu.Unlock()

			// Once released, break loop to finish
			break
		}

		return StatusEnd, nil
	}, StatusEnd).WithOptions(
		workflow.PollingFrequency(time.Millisecond * 10),
	)

	recordStore := memrecordstore.New()
	w := b.Build(
		memstreamer.New(),
		recordStore,
		memtimeoutstore.New(),
		memrolescheduler.New(),
		workflow.WithCustomDelete(func(object *myObject) error {
			object.Name = "Right to be forgotten"
			return nil
		}),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	foreignID := "foreignID"
	runID, err := w.Trigger(ctx, foreignID, StatusStart, workflow.WithInitialValue[myObject, status](&myObject{
		Name: "Andrew Wormald",
		Car:  "Audi",
	}))
	jtest.RequireNil(t, err)

	record, err := recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	time.Sleep(time.Millisecond * 500)

	record, err = recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, workflow.RunStateRunning, record.RunState)

	mu.Lock()
	canRelease = true
	mu.Unlock()

	rsc, err := w.RunStateController(ctx, foreignID, runID)
	jtest.RequireNil(t, err)

	_, err = rsc.Pause(ctx)
	jtest.RequireNil(t, err)

	record, err = recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, workflow.RunStatePaused, record.RunState)

	_, err = rsc.Resume(ctx)
	jtest.RequireNil(t, err)

	record, err = recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, workflow.RunStateRunning, record.RunState)

	_, err = rsc.Cancel(ctx)
	jtest.RequireNil(t, err)

	record, err = recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, workflow.RunStateCancelled, record.RunState)

	_, err = rsc.DeleteData(ctx)
	jtest.RequireNil(t, err)

	record, err = recordStore.Latest(ctx, w.Name, foreignID)
	jtest.RequireNil(t, err)

	require.Equal(t, workflow.RunStateDataDeleted, record.RunState)

	var object myObject
	err = workflow.Unmarshal(record.Object, &object)
	jtest.RequireNil(t, err)

	require.Equal(t, "Right to be forgotten", object.Name)
	require.Equal(t, "Audi", object.Car)

	w.Stop()
}

func TestIsFinished(t *testing.T) {
	require.False(t, workflow.RunStateInitiated.Finished())
	require.False(t, workflow.RunStateRunning.Finished())
	require.False(t, workflow.RunStatePaused.Finished())

	require.True(t, workflow.RunStateCompleted.Finished())
	require.True(t, workflow.RunStateCancelled.Finished())
	require.True(t, workflow.RunStateDataDeleted.Finished())
}
