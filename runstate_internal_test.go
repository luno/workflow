package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestingRunStateController(t *testing.T) {
	ctx := context.Background()

	t.Run("methods return nil error when no functions set", func(t *testing.T) {
		ctrl := testingRunStateController{}

		err := ctrl.Pause(ctx, "test reason")
		require.NoError(t, err)

		err = ctrl.Resume(ctx)
		require.NoError(t, err)

		err = ctrl.Cancel(ctx, "test reason")
		require.NoError(t, err)

		err = ctrl.DeleteData(ctx, "test reason")
		require.NoError(t, err)

		err = ctrl.SaveAndRepeat(ctx)
		require.NoError(t, err)
	})

	t.Run("methods call provided functions", func(t *testing.T) {
		var pauseCalled, resumeCalled, cancelCalled, deleteDataCalled, saveAndRepeatCalled bool

		ctrl := testingRunStateController{
			pause: func(ctx context.Context) error {
				pauseCalled = true
				return nil
			},
			resume: func(ctx context.Context) error {
				resumeCalled = true
				return nil
			},
			cancel: func(ctx context.Context) error {
				cancelCalled = true
				return nil
			},
			deleteData: func(ctx context.Context) error {
				deleteDataCalled = true
				return nil
			},
			saveAndRepeat: func(ctx context.Context) error {
				saveAndRepeatCalled = true
				return nil
			},
		}

		err := ctrl.Pause(ctx, "test reason")
		require.NoError(t, err)
		require.True(t, pauseCalled)

		err = ctrl.Resume(ctx)
		require.NoError(t, err)
		require.True(t, resumeCalled)

		err = ctrl.Cancel(ctx, "test reason")
		require.NoError(t, err)
		require.True(t, cancelCalled)

		err = ctrl.DeleteData(ctx, "test reason")
		require.NoError(t, err)
		require.True(t, deleteDataCalled)

		err = ctrl.SaveAndRepeat(ctx)
		require.NoError(t, err)
		require.True(t, saveAndRepeatCalled)
	})

	t.Run("methods propagate errors", func(t *testing.T) {
		testErr := errors.New("test error")

		ctrl := testingRunStateController{
			pause: func(ctx context.Context) error {
				return testErr
			},
			resume: func(ctx context.Context) error {
				return testErr
			},
			cancel: func(ctx context.Context) error {
				return testErr
			},
			deleteData: func(ctx context.Context) error {
				return testErr
			},
			saveAndRepeat: func(ctx context.Context) error {
				return testErr
			},
		}

		err := ctrl.Pause(ctx, "test reason")
		require.Equal(t, testErr, err)

		err = ctrl.Resume(ctx)
		require.Equal(t, testErr, err)

		err = ctrl.Cancel(ctx, "test reason")
		require.Equal(t, testErr, err)

		err = ctrl.DeleteData(ctx, "test reason")
		require.Equal(t, testErr, err)

		err = ctrl.SaveAndRepeat(ctx)
		require.Equal(t, testErr, err)
	})
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

			ctx := t.Context()
			err := ctrl.update(ctx, tc.to, "")
			require.Equal(t, tc.valid, err == nil)
		})
	}
}
