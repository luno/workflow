package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	werrors "github.com/luno/workflow/internal/errors"
)

func Test_trigger(t *testing.T) {
	b := NewBuilder[string, testStatus]("trigger test")
	b.AddStep(statusStart, func(ctx context.Context, r *Run[string, testStatus]) (testStatus, error) {
		return statusMiddle, nil
	}, statusMiddle)
	w := b.Build(nil, nil, nil, WithDebugMode())

	t.Run("Expected ErrWorkflowNotRunning when Trigger called before Run()", func(t *testing.T) {
		ctx := context.Background()
		_, err := trigger(ctx, w, nil, "1", statusStart)

		eval, is := err.(*werrors.Error)
		if is {
			fmt.Println(*eval)
		}

		require.Truef(t, errors.Is(err, ErrWorkflowNotRunning), "actual: %s", err.Error())
	})

	t.Run("Expects ErrStatusProvidedNotConfigured when starting status is not configured", func(t *testing.T) {
		ctx := context.Background()
		w.calledRun = true

		_, err := trigger(ctx, w, nil, "1", statusEnd)
		require.True(t, errors.Is(err, ErrStatusProvidedNotConfigured))
	})

	t.Run("Expects ErrWorkflowInProgress if a workflow run is already in progress", func(t *testing.T) {
		ctx := context.Background()
		w.calledRun = true

		_, err := trigger(ctx, w, func(ctx context.Context, workflowName, foreignID string) (*Record, error) {
			return &Record{
				ID:           1,
				WorkflowName: "trigger test",
				ForeignID:    "1",
				RunState:     RunStateRunning,
				Status:       int(statusMiddle),
			}, nil
		}, "1", statusStart)
		require.True(t, errors.Is(err, ErrWorkflowInProgress))
	})
}
