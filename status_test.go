package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)


// Note: The functions isSaveAndRepeat, skipUpdate, and skipUpdateDescription
// are internal/unexported functions, so their behavior is tested through
// integration tests in the workflow_test.go and update_test.go files.

func TestSkipTypeConstants(t *testing.T) {
	require.Equal(t, workflow.SkipType(0), workflow.SkipTypeDefault)
	require.Equal(t, workflow.SkipType(-1), workflow.SkipTypeRunStateUpdate)
	require.Equal(t, workflow.SkipType(-2), workflow.SkipTypeSaveAndRepeat)
}