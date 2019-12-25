package opts // import "github.com/docker/docker/opts"

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
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
