package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopRunStateController(t *testing.T) {
	ctrl := testingRunStateController{}

	ctx := context.Background()
	err := ctrl.Pause(ctx)
	require.Nil(t, err)

	err = ctrl.Resume(ctx)
	require.Nil(t, err)

	err = ctrl.Cancel(ctx)
	require.Nil(t, err)

	err = ctrl.DeleteData(ctx)
	require.Nil(t, err)
}

func TestRunStateControllerTransitions(t *testing.T) {
	testCases := []struct {
		name  string
		from  RunState
		to    RunState
		valid bool
	}{
		{
			name:  "Initiated to Running [valid]",
			from:  RunStateInitiated,
			to:    RunStateRunning,
			valid: true,
		},
		{
			name:  "Initiated to Paused [valid]",
			from:  RunStateInitiated,
			to:    RunStatePaused,
			valid: true,
		},
		{
			name:  "Initiated to Initiated [invalid]",
			from:  RunStateInitiated,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "Initiated to Cancelled [invalid]",
			from:  RunStateInitiated,
			to:    RunStateCancelled,
			valid: false,
		},
		{
			name:  "Initiated to Completed [invalid]",
			from:  RunStateInitiated,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "Initiated to DataDeleted [invalid]",
			from:  RunStateInitiated,
			to:    RunStateDataDeleted,
			valid: false,
		},
		{
			name:  "Initiated to RequestedDataDeleted [invalid]",
			from:  RunStateInitiated,
			to:    RunStateRequestedDataDeleted,
			valid: false,
		},
		{
			name:  "Running to Initiated [invalid]",
			from:  RunStateRunning,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "Running to Running [invalid]",
			from:  RunStateRunning,
			to:    RunStateRunning,
			valid: false,
		},
		{
			name:  "Running to Completed [valid]",
			from:  RunStateRunning,
			to:    RunStateCompleted,
			valid: true,
		},
		{
			name:  "Running to Paused [valid]",
			from:  RunStateRunning,
			to:    RunStatePaused,
			valid: true,
		},
		{
			name:  "Running to Cancelled [valid]",
			from:  RunStateRunning,
			to:    RunStateCancelled,
			valid: true,
		},
		{
			name:  "Running to RequestedDataDeleted [invalid]",
			from:  RunStateRunning,
			to:    RunStateRequestedDataDeleted,
			valid: false,
		},
		{
			name:  "Running to DataDeleted [invalid]",
			from:  RunStateRunning,
			to:    RunStateDataDeleted,
			valid: false,
		},
		{
			name:  "Paused to Initiated [invalid]",
			from:  RunStatePaused,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "Paused to Paused [invalid]",
			from:  RunStatePaused,
			to:    RunStatePaused,
			valid: false,
		},
		{
			name:  "Paused to Running [valid]",
			from:  RunStatePaused,
			to:    RunStateRunning,
			valid: true,
		},
		{
			name:  "Paused to Cancelled [valid]",
			from:  RunStatePaused,
			to:    RunStateCancelled,
			valid: true,
		},
		{
			name:  "Paused to Completed [invalid]",
			from:  RunStatePaused,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "Paused to RequestedDataDeleted [invalid]",
			from:  RunStatePaused,
			to:    RunStateRequestedDataDeleted,
			valid: false,
		},
		{
			name:  "Paused to DataDeleted [invalid]",
			from:  RunStatePaused,
			to:    RunStateDataDeleted,
			valid: false,
		},
		{
			name:  "Completed to RequestedDataDeleted [valid]",
			from:  RunStateCompleted,
			to:    RunStateRequestedDataDeleted,
			valid: true,
		},
		{
			name:  "Completed to DataDeleted [invalid]",
			from:  RunStateCompleted,
			to:    RunStateDataDeleted,
			valid: false,
		},
		{
			name:  "Completed to Initiated [invalid]",
			from:  RunStateCompleted,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "Completed to Running [invalid]",
			from:  RunStateCompleted,
			to:    RunStateRunning,
			valid: false,
		},
		{
			name:  "Completed to Paused [invalid]",
			from:  RunStateCompleted,
			to:    RunStatePaused,
			valid: false,
		},
		{
			name:  "Completed to Completed [invalid]",
			from:  RunStateCompleted,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "Completed to Cancelled [invalid]",
			from:  RunStateCompleted,
			to:    RunStateCancelled,
			valid: false,
		},
		{
			name:  "Cancelled to RequestedDataDeleted [valid]",
			from:  RunStateCancelled,
			to:    RunStateRequestedDataDeleted,
			valid: true,
		},
		{
			name:  "Cancelled to DataDeleted [invalid]",
			from:  RunStateCancelled,
			to:    RunStateDataDeleted,
			valid: false,
		},
		{
			name:  "Cancelled to Initiated [invalid]",
			from:  RunStateCancelled,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "Cancelled to Running [invalid]",
			from:  RunStateCancelled,
			to:    RunStateRunning,
			valid: false,
		},
		{
			name:  "Cancelled to Paused [invalid]",
			from:  RunStateCancelled,
			to:    RunStatePaused,
			valid: false,
		},
		{
			name:  "Cancelled to Completed [invalid]",
			from:  RunStateCancelled,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "Cancelled to Cancelled [invalid]",
			from:  RunStateCancelled,
			to:    RunStateCancelled,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to DataDeleted [valid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateDataDeleted,
			valid: true,
		},
		{
			name:  "RequestedDataDeleted to Initiated [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to Running [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateRunning,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to Paused [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStatePaused,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to Completed [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to Cancelled [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateCancelled,
			valid: false,
		},
		{
			name:  "RequestedDataDeleted to RequestedDataDeleted [invalid]",
			from:  RunStateRequestedDataDeleted,
			to:    RunStateRequestedDataDeleted,
			valid: false,
		},
		{
			name:  "DataDeleted to RequestedDataDeleted [valid]",
			from:  RunStateDataDeleted,
			to:    RunStateRequestedDataDeleted,
			valid: true,
		},
		{
			name:  "DataDeleted to Initiated [invalid]",
			from:  RunStateDataDeleted,
			to:    RunStateInitiated,
			valid: false,
		},
		{
			name:  "DataDeleted to Running [invalid]",
			from:  RunStateDataDeleted,
			to:    RunStateRunning,
			valid: false,
		},
		{
			name:  "DataDeleted to Paused [invalid]",
			from:  RunStateDataDeleted,
			to:    RunStatePaused,
			valid: false,
		},
		{
			name:  "DataDeleted to Completed [invalid]",
			from:  RunStateDataDeleted,
			to:    RunStateCompleted,
			valid: false,
		},
		{
			name:  "DataDeleted to Cancelled [invalid]",
			from:  RunStateDataDeleted,
			to:    RunStateCancelled,
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := runStateControllerImpl{
				record: &Record{
					RunState: tc.from,
				},
				store: func(ctx context.Context, record *Record) error {
					return nil
				},
			}

			ctx := context.Background()
			err := ctrl.update(ctx, tc.to)
			require.Equal(t, tc.valid, err == nil)
		})
	}
}
