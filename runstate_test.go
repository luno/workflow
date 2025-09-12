package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
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
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
				return StatusEnd, nil
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStateCompleted,
			},
		},
		{
			name: "Paused",
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
				return r.Pause(ctx, "testing pause")
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStatePaused,
			},
		},
		{
			name: "Cancelled",
			workflow: buildWorkflow(func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
				return r.Cancel(ctx, "testing cancel")
			}),
			expected: []workflow.RunState{
				workflow.RunStateInitiated,
				workflow.RunStateRunning,
				workflow.RunStateCancelled,
			},
		},
	}

	for _, tc := range testCases {
		fn := tc.workflow
		expected := tc.expected

		t.Run(tc.name, func(t *testing.T) {
			recordStore := memrecordstore.New()
			w := fn(recordStore)

			ctx := t.Context()
			w.Run(ctx)
			t.Cleanup(w.Stop)

			// Trigger workflow before it's running to assert that the initial state is workflow.RunStateInitiated
			runID, err := w.Trigger(ctx, "fid")
			require.NoError(t, err)

			time.Sleep(time.Second)

			snapshots := recordStore.Snapshots(w.Name(), "fid", runID)

			for i, state := range expected {
				require.Equal(t, state, snapshots[i].RunState)
			}
		})
	}
}

func buildWorkflow(fn workflow.ConsumerFunc[string, status]) func(recordStore workflow.RecordStore) *workflow.Workflow[string, status] {
	return func(recordStore workflow.RecordStore) *workflow.Workflow[string, status] {
		b := workflow.NewBuilder[string, status]("example")
		b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
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
			memrolescheduler.New(),
			workflow.WithDebugMode(),
		)

		return w
	}
}

func TestWorkflowRunStateController(t *testing.T) {
	type myObject struct {
		Name string
		Car  string
	}

	recordStore := memrecordstore.New()

	b, err := workflow.Marshal(&myObject{
		Name: "Andrew Wormald",
		Car:  "Audi",
	})
	require.NoError(t, err)

	ctx := t.Context()
	workflowName := "test-workflow"
	foreignID := "foreignID"
	runID := "uuid"
	err = recordStore.Store(ctx, &workflow.Record{
		WorkflowName: workflowName,
		ForeignID:    foreignID,
		RunID:        runID,
		RunState:     workflow.RunStateInitiated,
		Object:       b,
	})

	record, err := recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 500)

	record, err = recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	require.Equal(t, workflow.RunStateInitiated, record.RunState)

	wr, err := recordStore.Lookup(ctx, record.RunID)
	require.NoError(t, err)

	rsc := workflow.NewRunStateController(recordStore.Store, wr)

	err = rsc.Pause(ctx, "test pause")
	require.NoError(t, err)

	record, err = recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	require.Equal(t, workflow.RunStatePaused, record.RunState)
	require.Equal(t, "test pause", record.Meta.RunStateReason)

	err = rsc.Resume(ctx)
	require.NoError(t, err)

	record, err = recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	require.Equal(t, workflow.RunStateRunning, record.RunState)

	err = rsc.Cancel(ctx, "test cancel")
	require.NoError(t, err)

	record, err = recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	require.Equal(t, workflow.RunStateCancelled, record.RunState)
	require.Equal(t, "test cancel", record.Meta.RunStateReason)

	err = rsc.DeleteData(ctx, "test delete")
	require.NoError(t, err)

	record, err = recordStore.Latest(ctx, workflowName, foreignID)
	require.NoError(t, err)

	require.Equal(t, workflow.RunStateRequestedDataDeleted, record.RunState)
	require.Equal(t, "test delete", record.Meta.RunStateReason)
}

func TestIsFinished(t *testing.T) {
	require.False(t, workflow.RunStateInitiated.Finished())
	require.False(t, workflow.RunStateRunning.Finished())
	require.False(t, workflow.RunStatePaused.Finished())

	require.True(t, workflow.RunStateCompleted.Finished())
	require.True(t, workflow.RunStateCancelled.Finished())
	require.True(t, workflow.RunStateDataDeleted.Finished())
}

func TestRunStateValid(t *testing.T) {
	testCases := map[workflow.RunState]bool{
		workflow.RunState(-1):                     false,
		workflow.RunStateUnknown:                  false,
		workflow.RunStateInitiated:                true,
		workflow.RunStateRunning:                  true,
		workflow.RunStatePaused:                   true,
		workflow.RunStateCancelled:                true,
		workflow.RunStateCompleted:                true,
		workflow.RunStateDataDeleted:              true,
		workflow.RunStateRequestedDataDeleted:     true,
		workflow.RunStateRequestedDataDeleted + 1: false,
		workflow.RunStateRequestedDataDeleted + 2: false,
		workflow.RunState(9999):                   false,
	}

	for state, expected := range testCases {
		require.Equal(t, expected, state.Valid())
	}
}

func TestRunStateFinished(t *testing.T) {
	testCases := map[workflow.RunState]bool{
		workflow.RunState(-1):                     false,
		workflow.RunStateUnknown:                  false,
		workflow.RunStateInitiated:                false,
		workflow.RunStateRunning:                  false,
		workflow.RunStatePaused:                   false,
		workflow.RunStateCancelled:                true,
		workflow.RunStateCompleted:                true,
		workflow.RunStateDataDeleted:              true,
		workflow.RunStateRequestedDataDeleted:     true,
		workflow.RunStateRequestedDataDeleted + 1: false,
		workflow.RunStateRequestedDataDeleted + 2: false,
		workflow.RunState(9999):                   false,
	}

	for state, expected := range testCases {
		require.Equal(t, expected, state.Finished())
	}
}

func TestRunStateStopped(t *testing.T) {
	testCases := map[workflow.RunState]bool{
		workflow.RunState(-1):                     false,
		workflow.RunStateUnknown:                  false,
		workflow.RunStateInitiated:                false,
		workflow.RunStateRunning:                  false,
		workflow.RunStatePaused:                   true,
		workflow.RunStateCancelled:                true,
		workflow.RunStateCompleted:                false,
		workflow.RunStateDataDeleted:              true,
		workflow.RunStateRequestedDataDeleted:     true,
		workflow.RunStateRequestedDataDeleted + 1: false,
		workflow.RunStateRequestedDataDeleted + 2: false,
		workflow.RunState(9999):                   false,
	}

	for state, expected := range testCases {
		require.Equal(t, expected, state.Stopped())
	}
}

func TestRunStateOutOfBoundsString(t *testing.T) {
	require.Equal(t, "RunState(999)", workflow.RunState(999).String())
}
