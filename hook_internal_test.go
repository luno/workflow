package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_runHooks(t *testing.T) {
	ctx := context.Background()

	t.Run("Return non-nil error from lookup", func(t *testing.T) {
		testErr := errors.New("test error")
		err := runHook[string, testStatus](
			"workflow_name",
			"process_name",
			func(ctx context.Context, runID string) (*Record, error) {
				return nil, testErr
			},
			func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
				return nil
			},
		)(ctx, &Event{})
		require.Equal(t, testErr, err)
	})

	t.Run("Skip event on failure to unmarshal object", func(t *testing.T) {
		err := runHook[string, testStatus](
			"workflow_name",
			"process_name",
			func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					Object: []byte("INVALID JSON"),
				}, nil
			},
			func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
				return nil
			},
		)(ctx, &Event{})
		require.Nil(t, err)
	})

	t.Run("Return non-error if hook errors", func(t *testing.T) {
		testErr := errors.New("test error")
		err := runHook[string, testStatus](
			"workflow_name",
			"process_name",
			func(ctx context.Context, runID string) (*Record, error) {
				value := "data"
				b, err := Marshal(&value)
				require.Nil(t, err)

				current := &Record{
					WorkflowName: "example",
					ForeignID:    "32948623984623",
					RunID:        "JHFJDS-LSFKHJSLD-KSJDBLSL",
					RunState:     RunStateRunning,
					Status:       int(statusStart),
					Object:       b,
				}

				return current, nil
			},
			func(ctx context.Context, record *TypedRecord[string, testStatus]) error {
				return testErr
			},
		)(ctx, &Event{})
		require.Equal(t, testErr, err)
	})
}
