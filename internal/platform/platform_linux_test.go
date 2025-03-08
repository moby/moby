package platform

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParsePossibleCPUs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "Continuous Range",
			input:    "0-3",
			expected: []int{0, 1, 2, 3},
		},
		{
			name:     "Non-Continuous Range",
			input:    "0-2,4,6-7",
			expected: []int{0, 1, 2, 4, 6, 7},
		},
		{
			name:     "Single CPU",
			input:    "5",
			expected: []int{5},
		},
		{
			name:     "Empty Input",
			input:    "",
			expected: nil,
		},
		{
			name:     "Invalid Range",
			input:    "0-2,invalid",
			expected: nil,
		},
		{
			name:     "Malformed Range",
			input:    "0-2-3",
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := parsePossibleCPUs(test.input)
			assert.Assert(t, is.DeepEqual(result, test.expected), "Expected %v but got %v", test.expected, result)
		})
	}
}
