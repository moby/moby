package system

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestBuildEngineFeaturesHeader(t *testing.T) {
	testCases := []struct {
		doc      string
		in       map[string]bool
		expected string
	}{
		{
			doc:      "no features",
			in:       map[string]bool{},
			expected: "",
		},
		{
			doc: "single true",
			in: map[string]bool{
				"bork": true,
			},
			expected: "bork=true",
		},
		{
			doc: "single false",
			in: map[string]bool{
				"bork": false,
			},
			expected: "bork=false",
		},
		{
			doc: "multiple features",
			in: map[string]bool{
				"bork": true,
				"meow": false,
			},
			expected: "bork=true,meow=false",
		},
		{
			doc: "valid symbols",
			in: map[string]bool{
				"a?test/":       true,
				"another-+test": false,
			},
			expected: "a?test/=true,another-+test=false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			actual, err := buildEngineFeaturesHeader(tc.in)

			assert.NilError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
