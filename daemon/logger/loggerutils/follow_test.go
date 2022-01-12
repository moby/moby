package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"io"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestHandleDecoderErr(t *testing.T) {
	f, err := os.CreateTemp("", t.Name())
	assert.NilError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write([]byte("hello"))
	assert.NilError(t, err)

	pos, err := f.Seek(0, io.SeekCurrent)
	assert.NilError(t, err)
	assert.Assert(t, pos != 0)

	dec := &testDecoder{}

	// Simulate "turncate" case, where the file was bigger before.
	fl := &follow{file: f, dec: dec, oldSize: 100}
	err = fl.handleDecodeErr(io.EOF)
	assert.NilError(t, err)

	// handleDecodeErr seeks to zero.
	pos, err = f.Seek(0, io.SeekCurrent)
	assert.NilError(t, err)
	assert.Equal(t, int64(0), pos)

	// Reset is called.
	assert.Equal(t, 1, dec.resetCount)
}
