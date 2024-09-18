package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDetectContentType(t *testing.T) {
	input := []byte("That is just a plain text")

	contentType, err := detectContentType(input)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(mimeTypeTextPlain, contentType))
}
