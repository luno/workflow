package workflow_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestRecord(t *testing.T) {
	r := workflow.NewTestingRecord[string, status](t, workflow.WireRecord{}, "test")
	ctx := context.Background()

	pauseStatus, err := r.Pause(ctx)
	jtest.RequireNil(t, err)
	require.Equal(t, status(workflow.SkipTypeRunStateUpdate), pauseStatus)

	cancelStatus, err := r.Cancel(ctx)
	jtest.RequireNil(t, err)
	require.Equal(t, status(workflow.SkipTypeRunStateUpdate), cancelStatus)
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
