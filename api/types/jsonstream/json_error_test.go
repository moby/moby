package jsonstream

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestError(t *testing.T) {
	je := Error{404, "Not found"}
	assert.Assert(t, is.Error(&je, "Not found"))
}
