package jsonlog

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/timeutils"
)

func TestWriteLog(t *testing.T) {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	testLine := "Line that thinks that it is log line from docker\n"
	for i := 0; i < 30; i++ {
		e.Encode(JSONLog{Log: testLine, Stream: "stdout", Created: time.Now()})
	}
	w := bytes.NewBuffer(nil)
	format := timeutils.RFC3339NanoFixed
	if err := WriteLog(&buf, w, format, time.Time{}); err != nil {
		t.Fatal(err)
	}
	res := w.String()
	t.Logf("Result of WriteLog: %q", res)
	lines := strings.Split(strings.TrimSpace(res), "\n")
	if len(lines) != 30 {
		t.Fatalf("Must be 30 lines but got %d", len(lines))
	}
	// 30+ symbols, five more can come from system timezone
	logRe := regexp.MustCompile(`.{30,} Line that thinks that it is log line from docker`)
	for _, l := range lines {
		if !logRe.MatchString(l) {
			t.Fatalf("Log line not in expected format: %q", l)
		}
	}
}

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
		if err := WriteLog(r, w, format, time.Time{}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		r.Seek(0, 0)
		b.StartTimer()
	}
}
