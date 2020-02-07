package middleware // import "github.com/docker/docker/api/server/middleware"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMaskSecretKeys(t *testing.T) {
	tests := []struct {
		doc      string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			doc:      "secret/config create and update requests",
			input:    map[string]interface{}{"Data": "foo", "Name": "name", "Labels": map[string]interface{}{}},
			expected: map[string]interface{}{"Data": "*****", "Name": "name", "Labels": map[string]interface{}{}},
		},
		{
			doc: "masking other fields (recursively)",
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
			doc: "case insensitive field matching",
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
			maskSecretKeys(testcase.input)
			assert.Check(t, is.DeepEqual(testcase.expected, testcase.input))
		})
	}
}
