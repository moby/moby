package jsonlog

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func FuzzJSONLogsMarshalJSONBuf(f *testing.F) {
	f.Fuzz(func(t *testing.T, log []byte, stream string, created int64, rawAttrs []byte) {
		l := &JSONLogs{
			Log:      log,
			Stream:   stream,
			Created:  time.Unix(0, created),
			RawAttrs: json.RawMessage(rawAttrs),
		}
		// Exercise MarshalJSONBuf with arbitrary inputs. Errors are expected
		// for some generated times; the fuzz target is only checking that it
		// doesn't panic.
		var buf bytes.Buffer
		_ = l.MarshalJSONBuf(&buf)
	})
}
