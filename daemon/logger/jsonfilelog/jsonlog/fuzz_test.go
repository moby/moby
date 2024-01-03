package jsonlog

import (
	"bytes"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzJSONLogsMarshalJSONBuf(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		l := &JSONLogs{}
		err := ff.GenerateStruct(l)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		l.MarshalJSONBuf(&buf)
	})
}
