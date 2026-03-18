package jsonstream_test

import (
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestError(t *testing.T) {
	je := jsonstream.Error{Code: 404, Message: "Not found"}
	assert.Assert(t, is.Error(&je, "Not found"))
}

func TestNilError(t *testing.T) {
	var je *jsonstream.Error
	assert.Assert(t, is.Error(je, "<nil>"))
}
