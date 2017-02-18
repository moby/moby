package opts

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"testing"
)

func TestQuotedStringSetWithQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.NilError(t, qs.Set("\"something\""))
	assert.Equal(t, qs.String(), "something")
	assert.Equal(t, value, "something")
}

func TestQuotedStringSetWithMismatchedQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.NilError(t, qs.Set("\"something'"))
	assert.Equal(t, qs.String(), "\"something'")
}

func TestQuotedStringSetWithNoQuotes(t *testing.T) {
	value := ""
	qs := NewQuotedString(&value)
	assert.NilError(t, qs.Set("something"))
	assert.Equal(t, qs.String(), "something")
}
