package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrder(t *testing.T) {
	require.Equal(t, "", OrderTypeUnknown.String())
	require.Equal(t, "asc", OrderTypeAscending.String())
	require.Equal(t, "desc", OrderTypeDescending.String())
	require.Equal(t, "", sentinelOrderType.String())
}
