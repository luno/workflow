package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_trigger(t *testing.T) {
	b := NewBuilder[string, testStatus]("trigger test")
	b.AddStep(statusStart, func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
		return statusMiddle, nil
	}, statusMiddle)
	w := b.Build(nil, nil, nil, WithDebugMode())

	t.Run("Expected non-nil error when Trigger called before Run()", func(t *testing.T) {
		ctx := context.Background()
		_, err := trigger(ctx, w, nil, nil, "1")

		require.Equal(t, "trigger failed: workflow is not running", err.Error())
	})

	t.Run("Expects ErrStatusProvidedNotConfigured when starting status is not configured", func(t *testing.T) {
		ctx := context.Background()
		w.calledRun = true

		_, err := trigger(ctx, w, nil, nil, "1", WithStartingPoint[string, testStatus](statusEnd))
		require.Equal(t, fmt.Sprintf("trigger failed: status provided is not configured for workflow: %s", statusEnd), err.Error())
	})

	t.Run("Expects ErrWorkflowInProgress if a workflow run is already in progress", func(t *testing.T) {
		ctx := context.Background()
		w.calledRun = true

		_, err := trigger(ctx, w, func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return &Record{
				WorkflowName: "trigger test",
				ForeignID:    "1",
				RunState:     RunStateRunning,
				Status:       int(statusMiddle),
			}, nil
		}, nil, "1")
		require.True(t, errors.Is(err, ErrWorkflowInProgress))
	})

	t.Run("Expects Meta to be correctly set", func(t *testing.T) {
		ctx := context.Background()
		w.calledRun = true

		_, err := trigger(ctx, w, func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return nil, ErrRecordNotFound
		}, func(ctx context.Context, record *Record) error {
			require.Equal(t, uint(1), record.Meta.Version)
			require.Equal(t, statusStart.String(), record.Meta.StatusDescription)
			require.Contains(t, record.Meta.TraceOrigin, "trigger_internal_test.go")
			return nil
		}, "1")
		require.NoError(t, err)
	})
}
