package opts // import "github.com/docker/docker/opts"

import (
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
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
