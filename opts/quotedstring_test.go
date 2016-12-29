package opts

import (
	"github.com/docker/docker/pkg/testutil/assert"
	"testing"
)

func TestQuotedStringSetWithQuotes(t *testing.T) {
	qs := QuotedString("")
	assert.NilError(t, qs.Set("\"something\""))
	assert.Equal(t, qs.String(), "something")
}

func TestQuotedStringSetWithMismatchedQuotes(t *testing.T) {
	qs := QuotedString("")
	assert.NilError(t, qs.Set("\"something'"))
	assert.Equal(t, qs.String(), "\"something'")
}

func TestQuotedStringSetWithNoQuotes(t *testing.T) {
	qs := QuotedString("")
	assert.NilError(t, qs.Set("something"))
	assert.Equal(t, qs.String(), "something")
}
