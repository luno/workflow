package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestUpdateType_String(t *testing.T) {
	tests := []struct {
		name     string
		ut       workflow.UpdateType
		expected string
	}{
		{
			name:     "Default",
			ut:       workflow.UpdateTypeDefault,
			expected: "Default",
		},
		{
			name:     "State Only",
			ut:       workflow.UpdateTypeStateOnly,
			expected: "State Only",
		},
		{
			name:     "Unknown value",
			ut:       workflow.UpdateType(99),
			expected: "UpdateType(99)",
		},
		{
			name:     "Negative unknown value",
			ut:       workflow.UpdateType(-10),
			expected: "UpdateType(-10)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ut.String()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateType_Constants(t *testing.T) {
	require.Equal(t, workflow.UpdateTypeDefault, workflow.UpdateType(0))
	require.Equal(t, workflow.UpdateTypeStateOnly, workflow.UpdateType(1))
}