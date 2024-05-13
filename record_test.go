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
