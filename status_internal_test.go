package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkipTypeConstants(t *testing.T) {
	require.Equal(t, skipType(0), skipTypeDefault)
	require.Equal(t, skipType(-1), skipTypeRunStateUpdate)
	require.Equal(t, skipType(-2), skipTypeSaveAndRepeat)
}
