package local

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/logger/internal/logdriver"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestDecodeIncompleteRecord verifies that Decode returns io.EOF when the
// underlying file ends in the middle of a record, and that decoding succeeds
// once the remainder of the record is appended.
func TestDecodeIncompleteRecord(t *testing.T) {
	buf := make([]byte, 0)

	extraAttrs := []*logdriver.LogAttr{{Key: "a", Value: "b"}}
	err := marshal(&logger.Message{Line: []byte("hello")}, extraAttrs, &buf)
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	for split := range len(buf) {
		t.Run("split="+strconv.Itoa(split), func(t *testing.T) {
			logFile := filepath.Join(tmpDir, "log_"+strconv.Itoa(split)+".log")
			fw, err := os.Create(logFile)
			assert.NilError(t, err)
			t.Cleanup(func() { assert.NilError(t, fw.Close()) })

			fr, err := os.Open(logFile)
			assert.NilError(t, err)
			t.Cleanup(func() { assert.NilError(t, fr.Close()) })

			d := &decoder{rdr: fr}

			if split > 0 {
				_, err = fw.Write(buf[0:split])
				assert.NilError(t, err)

				_, err = d.Decode()
				assert.ErrorIs(t, err, io.EOF)

				_, err = fw.Write(buf[split:])
				assert.NilError(t, err)
			} else {
				_, err = fw.Write(buf)
				assert.NilError(t, err)
			}

			msg, err := d.Decode()
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(msg.Line), "hello\n"))
			assert.Check(t, is.Len(msg.Attrs, 1))
			assert.Check(t, is.Equal(msg.Attrs[0].Key, "a"))
			assert.Check(t, is.Equal(msg.Attrs[0].Value, "b"))
		})
	}
}
