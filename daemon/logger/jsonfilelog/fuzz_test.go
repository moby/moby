package jsonfilelog

import (
	"bytes"
	"testing"
)

func FuzzLoggerDecode(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		dec := decodeFunc(bytes.NewBuffer(data))
		defer dec.Close()

		_, _ = dec.Decode()
	})
}
