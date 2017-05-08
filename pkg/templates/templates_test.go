package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewParse(t *testing.T) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	assert.NoError(t, err)

	var b bytes.Buffer
	assert.NoError(t, tm.Execute(&b, "string"))
	want := "this is a string"
	assert.Equal(t, want, b.String())
}

func TestParseStringTruncateFunction(t *testing.T) {
	testCases := []struct {
		template string
		source   string
		expected string
	}{
		{
			template: `{{join (split . ":") "/"}}`,
			source:   "text:with:colon",
			expected: "text/with/colon",
		},
		{
			template: `{{replace . ":" "/" -1}}`,
			source:   "text:with:colon",
			expected: "text/with/colon",
		},
		{
			template: `{{upper . }}`,
			source:   "abc12",
			expected: "ABC12",
		},
		{
			template: `{{lower . }}`,
			source:   "ABC12",
			expected: "abc12",
		},
		{
			template: `{{title . }}`,
			source:   "her royal highness",
			expected: "Her Royal Highness",
		},
		{
			template: `{{pad . 2 3}}`,
			source:   "",
			expected: "",
		},
		{
			template: `{{pad . 2 3}}`,
			source:   "padthis",
			expected: "  padthis   ",
		},
		{
			template: `{{truncate . 5}}`,
			source:   "tupx5xzf6hvsrhnruz5cr8gwp",
			expected: "tupx5",
		},
		{
			template: `{{truncate . 25}}`,
			source:   "tupx5xzf6hvsrhnruz5cr8gwp",
			expected: "tupx5xzf6hvsrhnruz5cr8gwp",
		},
		{
			template: `{{truncate . 30}}`,
			source:   "tupx5xzf6hvsrhnruz5cr8gwp",
			expected: "tupx5xzf6hvsrhnruz5cr8gwp",
		},
	}

	for _, testCase := range testCases {
		tm, err := Parse(testCase.template)
		assert.NoError(t, err)

		var b bytes.Buffer
		assert.NoError(t, tm.Execute(&b, testCase.source))
		assert.Equal(t, testCase.expected, b.String())
	}
}
