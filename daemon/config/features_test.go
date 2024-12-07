package config

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestPopulateFeatures(t *testing.T) {
	testCases := []struct {
		doc             string
		in              map[string]bool
		defaultFeatures map[string]bool
		expected        map[string]bool
		expectedError   string
	}{
		{
			doc:             "empty",
			in:              map[string]bool{},
			defaultFeatures: map[string]bool{},
			expected:        map[string]bool{},
		},
		{
			doc: "no default",
			in: map[string]bool{
				"foo": true,
			},
			defaultFeatures: map[string]bool{},
			expected: map[string]bool{
				"foo": true,
			},
		},
		{
			doc: "just defaults",
			in:  map[string]bool{},
			defaultFeatures: map[string]bool{
				"foo": false,
			},
			expected: map[string]bool{
				"foo": false,
			},
		},
		{
			doc: "multiple defaults",
			in:  map[string]bool{},
			defaultFeatures: map[string]bool{
				"foo": false,
				"bar": true,
			},
			expected: map[string]bool{
				"foo": false,
				"bar": true,
			},
		},
		{
			doc: "defaults don't overwrite config",
			in: map[string]bool{
				"foo": false,
				"bar": false,
			},
			defaultFeatures: map[string]bool{
				"foo": true,
				"bar": true,
			},
			expected: map[string]bool{
				"foo": false,
				"bar": false,
			},
		},
		{
			doc: "config + defaults",
			in: map[string]bool{
				"foo": false,
			},
			defaultFeatures: map[string]bool{
				"bar": true,
			},
			expected: map[string]bool{
				"foo": false,
				"bar": true,
			},
		},
		{
			doc: "nil features",
			in:  nil,
			defaultFeatures: map[string]bool{
				"bar": true,
			},
			expectedError: "features cannot be nil",
		},
		{
			doc: "nil defaults",
			in: map[string]bool{
				"foo": true,
			},
			defaultFeatures: nil,
			expectedError:   "DefaultFeatures cannot be nil",
		},
		{
			doc:             "both nil",
			in:              nil,
			defaultFeatures: nil,
			expectedError:   "features cannot be nil",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			features := tc.in
			DefaultFeatures = tc.defaultFeatures

			err := PopulateFeatures(features)

			if tc.expectedError == "" {
				assert.DeepEqual(t, features, tc.expected)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}
