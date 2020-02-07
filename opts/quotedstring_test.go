package opts // import "github.com/docker/docker/opts"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestQuotedStringSetWithQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.Check(t, qs.Set(`"something"`))
	assert.Check(t, is.Equal("something", qs.String()))
	assert.Check(t, is.Equal("something", value))
}

func TestQuotedStringSetWithMismatchedQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.Check(t, qs.Set(`"something'`))
	assert.Check(t, is.Equal(`"something'`, qs.String()))
}

func TestQuotedStringSetWithNoQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.Check(t, qs.Set("something"))
	assert.Check(t, is.Equal("something", qs.String()))
}
