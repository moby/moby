package jsonlog

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/docker/docker/pkg/timeutils"
)

func BenchmarkWriteLog(b *testing.B) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	testLine := "Line that thinks that it is log line from docker\n"
	for i := 0; i < 30; i++ {
		e.Encode(JSONLog{Log: testLine, Stream: "stdout", Created: time.Now()})
	}
	r := bytes.NewReader(buf.Bytes())
	w := ioutil.Discard
	format := timeutils.RFC3339NanoFixed
	b.SetBytes(int64(r.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteLog(r, w, format); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		r.Seek(0, 0)
		b.StartTimer()
	}
}
