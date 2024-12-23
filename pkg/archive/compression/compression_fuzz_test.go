package compression

import (
	"bytes"
	"testing"
)

func FuzzDecompressStream(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, _ = DecompressStream(r)
	})
}
