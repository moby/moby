package templates

import (
	"bytes"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestParseStringFunctions(t *testing.T) {
	tm, err := Parse(`{{join (split . ":") "/"}}`)
	assert.NilError(t, err)

	var b bytes.Buffer
	assert.NilError(t, tm.Execute(&b, "text:with:colon"))
	want := "text/with/colon"
	assert.Equal(t, b.String(), want)
}

func TestNewParse(t *testing.T) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	assert.NilError(t, err)

	var b bytes.Buffer
	assert.NilError(t, tm.Execute(&b, "string"))
	want := "this is a string"
	assert.Equal(t, b.String(), want)
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
		assert.NilError(t, err)

		var b bytes.Buffer
		assert.NilError(t, tm.Execute(&b, source))
		assert.Equal(t, b.String(), testCase.expected)
	}
}
