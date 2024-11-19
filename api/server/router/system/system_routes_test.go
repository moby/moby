package system

import (
	"fmt"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

func TestBuildEngineFeaturesHeader(t *testing.T) {
	testCases := []struct {
		doc           string
		in            map[string]bool
		expected      string
		expectedError string
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
		{
			doc: "invalid feature key – equals '='",
			in: map[string]bool{
				"foo=bar": true,
			},
			expectedError: "invalid feature – key cannot contain '=': foo=bar",
		},
		{
			doc: "invalid feature key – comma ','",
			in: map[string]bool{
				"a,comma": true,
			},
			expectedError: "invalid feature – key cannot contain ',': a,comma",
		},
		{
			doc: "valid and invalid features",
			in: map[string]bool{
				"bork":         true,
				"invalid=meow": false,
			},
			expectedError: "invalid feature – key cannot contain '=': invalid=meow",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			actual, err := buildEngineFeaturesHeader(tc.in)

			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.Equal(t, tc.expected, actual)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
				assert.Equal(t, actual, "")
			}
		})
	}
}

func TestBuildEngineFeaturesHeaderSizeLimits(t *testing.T) {
	t.Run("feature key too long", func(t *testing.T) {
		const longFeatureName = "1234567890123456789012345678901234567890"
		in := map[string]bool{
			longFeatureName: true,
		}

		actual, err := buildEngineFeaturesHeader(in)

		expectedError := fmt.Sprintf("feature name length cannot be over %d: %s", maxFeatureKeyLen, longFeatureName)
		assert.ErrorContains(t, err, expectedError)
		assert.Equal(t, actual, "")
	})

	t.Run("too many features", func(t *testing.T) {
		in := make(map[string]bool)
		for i := 0; i < 101; i++ {
			featureName := strconv.Itoa(i)
			in[featureName] = true
		}

		actual, err := buildEngineFeaturesHeader(in)

		expectedError := fmt.Sprintf("too many features – expected max %d, found %d", maxFeatures, 101)
		assert.ErrorContains(t, err, expectedError)
		assert.Equal(t, actual, "")
	})
}
