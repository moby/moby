package streamformatter // import "github.com/docker/docker/pkg/streamformatter"

import (
	"bytes"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestStreamWriterStdout(t *testing.T) {
	buffer := &bytes.Buffer{}
	content := "content"
	sw := NewStdoutWriter(buffer)
	size, err := sw.Write([]byte(content))

	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(content), size))

	expected := `{"stream":"content"}` + streamNewline
	assert.Check(t, is.Equal(expected, buffer.String()))
}

func TestStreamWriterStderr(t *testing.T) {
	buffer := &bytes.Buffer{}
	content := "content"
	sw := NewStderrWriter(buffer)
	size, err := sw.Write([]byte(content))

	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(content), size))

	expected := `{"stream":"\u001b[91mcontent\u001b[0m"}` + streamNewline
	assert.Check(t, is.Equal(expected, buffer.String()))
}
