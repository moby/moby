package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestMaskSecretKeys(t *testing.T) {
	tests := []struct {
		doc      string
		path     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			doc:      "secret create with API version",
			path:     "/v1.30/secrets/create",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			doc:      "secret create with API version and trailing slashes",
			path:     "/v1.30/secrets/create//",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			doc:      "secret create with query param",
			path:     "/secrets/create?key=val",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			doc:  "other paths with API version",
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
			doc:  "other paths with API version case insensitive",
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
		t.Run(testcase.doc, func(t *testing.T) {
			maskSecretKeys(testcase.input, testcase.path)
			assert.Check(t, is.DeepEqual(testcase.expected, testcase.input))
		})
	}
}
