package local

import (
	"io"
	"os"
	"testing"

	"github.com/docker/docker/daemon/logger"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

func TestDecode(t *testing.T) {
	buf := make([]byte, 0)

	err := marshal(&logger.Message{Line: []byte("hello")}, &buf)
	assert.NilError(t, err)

	for i := 0; i < len(buf); i++ {
		testDecode(t, buf, i)
	}
}

func testDecode(t *testing.T, buf []byte, split int) {
	fw, err := os.CreateTemp("", t.Name())
	assert.NilError(t, err)
	defer os.Remove(fw.Name())

	fr, err := os.Open(fw.Name())
	assert.NilError(t, err)

	d := &decoder{rdr: fr}

	if split > 0 {
		_, err = fw.Write(buf[0:split])
		assert.NilError(t, err)

		_, err = d.Decode()
		assert.Assert(t, errors.Is(err, io.EOF))

		_, err = fw.Write(buf[split:])
		assert.NilError(t, err)
	} else {
		_, err = fw.Write(buf)
		assert.NilError(t, err)
	}

	message, err := d.Decode()
	assert.NilError(t, err)
	assert.Equal(t, "hello\n", string(message.Line))
}
