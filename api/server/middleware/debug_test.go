package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestMaskSecretKeys(t *testing.T) {
	tests := []struct {
		path     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			path:     "/v1.30/secrets/create",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			path:     "/v1.30/secrets/create//",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			path:     "/secrets/create?key=val",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			path: "/v1.30/some/other/path",
			input: map[string]interface{}{
				"password":     "pass",
				"secret":       "secret",
				"jointoken":    "jointoken",
				"unlockkey":    "unlockkey",
				"signingcakey": "signingcakey",
				"other": map[string]interface{}{
					"password":     "pass",
					"secret":       "secret",
					"jointoken":    "jointoken",
					"unlockkey":    "unlockkey",
					"signingcakey": "signingcakey",
				},
			},
			expected: map[string]interface{}{
				"password":     "*****",
				"secret":       "*****",
				"jointoken":    "*****",
				"unlockkey":    "*****",
				"signingcakey": "*****",
				"other": map[string]interface{}{
					"password":     "*****",
					"secret":       "*****",
					"jointoken":    "*****",
					"unlockkey":    "*****",
					"signingcakey": "*****",
				},
			},
		},
		{
			path: "/v1.30/some/other/path",
			input: map[string]interface{}{
				"PASSWORD": "pass",
				"other": map[string]interface{}{
					"PASSWORD": "pass",
				},
			},
			expected: map[string]interface{}{
				"PASSWORD": "*****",
				"other": map[string]interface{}{
					"PASSWORD": "*****",
				},
			},
		},
	}

	for _, testcase := range tests {
		maskSecretKeys(testcase.input, testcase.path)
		assert.Check(t, is.DeepEqual(testcase.expected, testcase.input))
	}
}
