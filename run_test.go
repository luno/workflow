package workflow_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestNewTestingRun(t *testing.T) {
	r := workflow.NewTestingRun[string, status](t, workflow.Record{}, "test")
	ctx := context.Background()

	pauseStatus, err := r.Pause(ctx)
	require.Nil(t, err)
	require.Equal(t, status(workflow.SkipTypeRunStateUpdate), pauseStatus)

	cancelStatus, err := r.Cancel(ctx)
	require.Nil(t, err)
	require.Equal(t, status(workflow.SkipTypeRunStateUpdate), cancelStatus)
}
