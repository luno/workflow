package util_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/util"
)

func TestCamelToSpacing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"camelCase", "camel Case"},
		{"thisIsATest", "this Is A Test"},
		{"helloWorld", "hello World"},
		{"singleWord", "single Word"},
		{"Lowercase", "Lowercase"}, // No change if no camel case
		{"", ""},                   // Empty string case
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("case %v: %v", i+1, test.input), func(t *testing.T) {
			result := util.CamelToSpacing(test.input)
			require.Equal(t, test.expected, result)
		})
	}
}
