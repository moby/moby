package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Github #32120
func TestParseJSONFunctions(t *testing.T) {
	tm, err := Parse(`{{json .Ports}}`)
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, tm.Execute(&b, map[string]string{"Ports": "0.0.0.0:2->8/udp"}))
	want := "\"0.0.0.0:2->8/udp\""
	assert.Equal(t, want, b.String())
}

func TestParseStringFunctions(t *testing.T) {
	tm, err := Parse(`{{join (split . ":") "/"}}`)
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, tm.Execute(&b, "text:with:colon"))
	want := "text/with/colon"
	assert.Equal(t, want, b.String())
}

func TestNewParse(t *testing.T) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, tm.Execute(&b, "string"))
	want := "this is a string"
	assert.Equal(t, want, b.String())
}

func TestParseTruncateFunction(t *testing.T) {
	source := "tupx5xzf6hvsrhnruz5cr8gwp"

	testCases := []struct {
		template string
		expected string
	}{
		{
			template: `{{truncate . 5}}`,
			expected: "tupx5",
		},
		{
			template: `{{truncate . 25}}`,
			expected: "tupx5xzf6hvsrhnruz5cr8gwp",
		},
		{
			template: `{{truncate . 30}}`,
			expected: "tupx5xzf6hvsrhnruz5cr8gwp",
		},
	}

	for _, testCase := range testCases {
		tm, err := Parse(testCase.template)
		assert.NoError(t, err)

		var b bytes.Buffer
		assert.NoError(t, tm.Execute(&b, source))
		assert.Equal(t, testCase.expected, b.String())
	}
}
