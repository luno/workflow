package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTestingRun(t *testing.T) {
	r := NewTestingRun[string, testStatus](t, Record{}, "test")
	ctx := t.Context()

	pauseStatus, err := r.Pause(ctx, "")
	require.NoError(t, err)
	require.Equal(t, testStatus(skipTypeRunStateUpdate), pauseStatus)

	cancelStatus, err := r.Cancel(ctx, "")
	require.NoError(t, err)
	require.Equal(t, testStatus(skipTypeRunStateUpdate), cancelStatus)
}

func TestNewTestingRun_requiresTestingParam(t *testing.T) {
	require.PanicsWithValue(t,
		"Cannot use NewTestingRun without *testing.T parameter",
		func() {
			_ = NewTestingRun[string, testStatus](nil, Record{}, "test")
		},
	)
}
