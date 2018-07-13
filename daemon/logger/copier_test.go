package logger // import "github.com/docker/docker/daemon/logger"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type TestLoggerJSON struct {
	*json.Encoder
	mu    sync.Mutex
	delay time.Duration
}

func (l *TestLoggerJSON) Log(m *Message) error {
	if l.delay > 0 {
		time.Sleep(l.delay)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Encode(m)
}

func (l *TestLoggerJSON) Close() error { return nil }

func (l *TestLoggerJSON) Name() string { return "json" }

type TestSizedLoggerJSON struct {
	*json.Encoder
	mu sync.Mutex
}

func (l *TestSizedLoggerJSON) Log(m *Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Encode(m)
}

func (*TestSizedLoggerJSON) Close() error { return nil }

func (*TestSizedLoggerJSON) Name() string { return "sized-json" }

func (*TestSizedLoggerJSON) BufSize() int {
	return 32 * 1024
}

func TestCopier(t *testing.T) {
	stdoutLine := "Line that thinks that it is log line from docker stdout"
	stderrLine := "Line that thinks that it is log line from docker stderr"
	stdoutTrailingLine := "stdout trailing line"
	stderrTrailingLine := "stderr trailing line"

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for i := 0; i < 30; i++ {
		if _, err := stdout.WriteString(stdoutLine + "\n"); err != nil {
			t.Fatal(err)
		}
		if _, err := stderr.WriteString(stderrLine + "\n"); err != nil {
			t.Fatal(err)
		}
	}

	// Test remaining lines without line-endings
	if _, err := stdout.WriteString(stdoutTrailingLine); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.WriteString(stderrTrailingLine); err != nil {
		t.Fatal(err)
	}

	var jsonBuf bytes.Buffer

	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf)}

	c := NewCopier(
		map[string]io.Reader{
			"stdout": &stdout,
			"stderr": &stderr,
		},
		jsonLog)
	c.Run()
	wait := make(chan struct{})
	go func() {
		c.Wait()
		close(wait)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("Copier failed to do its work in 1 second")
	case <-wait:
	}
	dec := json.NewDecoder(&jsonBuf)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if msg.Source != "stdout" && msg.Source != "stderr" {
			t.Fatalf("Wrong Source: %q, should be %q or %q", msg.Source, "stdout", "stderr")
		}
		if msg.Source == "stdout" {
			if string(msg.Line) != stdoutLine && string(msg.Line) != stdoutTrailingLine {
				t.Fatalf("Wrong Line: %q, expected %q or %q", msg.Line, stdoutLine, stdoutTrailingLine)
			}
		}
		if msg.Source == "stderr" {
			if string(msg.Line) != stderrLine && string(msg.Line) != stderrTrailingLine {
				t.Fatalf("Wrong Line: %q, expected %q or %q", msg.Line, stderrLine, stderrTrailingLine)
			}
		}
	}
}

// TestCopierLongLines tests long lines without line breaks
func TestCopierLongLines(t *testing.T) {
	// Long lines (should be split at "defaultBufSize")
	stdoutLongLine := strings.Repeat("a", defaultBufSize)
	stderrLongLine := strings.Repeat("b", defaultBufSize)
	stdoutTrailingLine := "stdout trailing line"
	stderrTrailingLine := "stderr trailing line"

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	for i := 0; i < 3; i++ {
		if _, err := stdout.WriteString(stdoutLongLine); err != nil {
			t.Fatal(err)
		}
		if _, err := stderr.WriteString(stderrLongLine); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := stdout.WriteString(stdoutTrailingLine); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.WriteString(stderrTrailingLine); err != nil {
		t.Fatal(err)
	}

	var jsonBuf bytes.Buffer

	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf)}

	c := NewCopier(
		map[string]io.Reader{
			"stdout": &stdout,
			"stderr": &stderr,
		},
		jsonLog)
	c.Run()
	wait := make(chan struct{})
	go func() {
		c.Wait()
		close(wait)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("Copier failed to do its work in 1 second")
	case <-wait:
	}
	dec := json.NewDecoder(&jsonBuf)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if msg.Source != "stdout" && msg.Source != "stderr" {
			t.Fatalf("Wrong Source: %q, should be %q or %q", msg.Source, "stdout", "stderr")
		}
		if msg.Source == "stdout" {
			if string(msg.Line) != stdoutLongLine && string(msg.Line) != stdoutTrailingLine {
				t.Fatalf("Wrong Line: %q, expected 'stdoutLongLine' or 'stdoutTrailingLine'", msg.Line)
			}
		}
		if msg.Source == "stderr" {
			if string(msg.Line) != stderrLongLine && string(msg.Line) != stderrTrailingLine {
				t.Fatalf("Wrong Line: %q, expected 'stderrLongLine' or 'stderrTrailingLine'", msg.Line)
			}
		}
	}
}

func TestCopierSlow(t *testing.T) {
	stdoutLine := "Line that thinks that it is log line from docker stdout"
	var stdout bytes.Buffer
	for i := 0; i < 30; i++ {
		if _, err := stdout.WriteString(stdoutLine + "\n"); err != nil {
			t.Fatal(err)
		}
	}

	var jsonBuf bytes.Buffer
	//encoder := &encodeCloser{Encoder: json.NewEncoder(&jsonBuf)}
	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf), delay: 100 * time.Millisecond}

	c := NewCopier(map[string]io.Reader{"stdout": &stdout}, jsonLog)
	c.Run()
	wait := make(chan struct{})
	go func() {
		c.Wait()
		close(wait)
	}()
	<-time.After(150 * time.Millisecond)
	c.Close()
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("failed to exit in time after the copier is closed")
	case <-wait:
	}
}

func TestCopierWithSized(t *testing.T) {
	var jsonBuf bytes.Buffer
	expectedMsgs := 2
	sizedLogger := &TestSizedLoggerJSON{Encoder: json.NewEncoder(&jsonBuf)}
	logbuf := bytes.NewBufferString(strings.Repeat(".", sizedLogger.BufSize()*expectedMsgs))
	c := NewCopier(map[string]io.Reader{"stdout": logbuf}, sizedLogger)

	c.Run()
	// Wait for Copier to finish writing to the buffered logger.
	c.Wait()
	c.Close()

	recvdMsgs := 0
	dec := json.NewDecoder(&jsonBuf)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if msg.Source != "stdout" {
			t.Fatalf("Wrong Source: %q, should be %q", msg.Source, "stdout")
		}
		if len(msg.Line) != sizedLogger.BufSize() {
			t.Fatalf("Line was not of expected max length %d, was %d", sizedLogger.BufSize(), len(msg.Line))
		}
		recvdMsgs++
	}
	if recvdMsgs != expectedMsgs {
		t.Fatalf("expected to receive %d messages, actually received %d", expectedMsgs, recvdMsgs)
	}
}

func checkIdentical(t *testing.T, msg Message, expectedID string, expectedTS time.Time) {
	if msg.PLogMetaData.ID != expectedID {
		t.Fatalf("IDs are not he same across partials. Expected: %s Received: %s",
			expectedID, msg.PLogMetaData.ID)
	}
	if msg.Timestamp != expectedTS {
		t.Fatalf("Timestamps are not the same across partials. Expected: %v Received: %v",
			expectedTS.Format(time.UnixDate), msg.Timestamp.Format(time.UnixDate))
	}
}

// Have long lines and make sure that it comes out with PartialMetaData
func TestCopierWithPartial(t *testing.T) {
	stdoutLongLine := strings.Repeat("a", defaultBufSize)
	stderrLongLine := strings.Repeat("b", defaultBufSize)
	stdoutTrailingLine := "stdout trailing line"
	stderrTrailingLine := "stderr trailing line"
	normalStr := "This is an impartial message :)"

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var normalMsg bytes.Buffer

	for i := 0; i < 3; i++ {
		if _, err := stdout.WriteString(stdoutLongLine); err != nil {
			t.Fatal(err)
		}
		if _, err := stderr.WriteString(stderrLongLine); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := stdout.WriteString(stdoutTrailingLine + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.WriteString(stderrTrailingLine + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := normalMsg.WriteString(normalStr + "\n"); err != nil {
		t.Fatal(err)
	}

	var jsonBuf bytes.Buffer

	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf)}

	c := NewCopier(
		map[string]io.Reader{
			"stdout": &stdout,
			"normal": &normalMsg,
			"stderr": &stderr,
		},
		jsonLog)
	c.Run()
	wait := make(chan struct{})
	go func() {
		c.Wait()
		close(wait)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("Copier failed to do its work in 1 second")
	case <-wait:
	}

	dec := json.NewDecoder(&jsonBuf)
	expectedMsgs := 9
	recvMsgs := 0
	var expectedPartID1, expectedPartID2 string
	var expectedTS1, expectedTS2 time.Time

	for {
		var msg Message

		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if msg.Source != "stdout" && msg.Source != "stderr" && msg.Source != "normal" {
			t.Fatalf("Wrong Source: %q, should be %q or %q or %q", msg.Source, "stdout", "stderr", "normal")
		}

		if msg.Source == "stdout" {
			if string(msg.Line) != stdoutLongLine && string(msg.Line) != stdoutTrailingLine {
				t.Fatalf("Wrong Line: %q, expected 'stdoutLongLine' or 'stdoutTrailingLine'", msg.Line)
			}

			if msg.PLogMetaData.ID == "" {
				t.Fatalf("Expected partial metadata. Got nothing")
			}

			if msg.PLogMetaData.Ordinal == 1 {
				expectedPartID1 = msg.PLogMetaData.ID
				expectedTS1 = msg.Timestamp
			} else {
				checkIdentical(t, msg, expectedPartID1, expectedTS1)
			}
			if msg.PLogMetaData.Ordinal == 4 && !msg.PLogMetaData.Last {
				t.Fatalf("Last is not set for last chunk")
			}
		}

		if msg.Source == "stderr" {
			if string(msg.Line) != stderrLongLine && string(msg.Line) != stderrTrailingLine {
				t.Fatalf("Wrong Line: %q, expected 'stderrLongLine' or 'stderrTrailingLine'", msg.Line)
			}

			if msg.PLogMetaData.ID == "" {
				t.Fatalf("Expected partial metadata. Got nothing")
			}

			if msg.PLogMetaData.Ordinal == 1 {
				expectedPartID2 = msg.PLogMetaData.ID
				expectedTS2 = msg.Timestamp
			} else {
				checkIdentical(t, msg, expectedPartID2, expectedTS2)
			}
			if msg.PLogMetaData.Ordinal == 4 && !msg.PLogMetaData.Last {
				t.Fatalf("Last is not set for last chunk")
			}
		}

		if msg.Source == "normal" && msg.PLogMetaData != nil {
			t.Fatalf("Normal messages should not have PartialLogMetaData")
		}
		recvMsgs++
	}

	if expectedMsgs != recvMsgs {
		t.Fatalf("Expected msgs: %d Recv msgs: %d", expectedMsgs, recvMsgs)
	}
}

type BenchmarkLoggerDummy struct {
}

func (l *BenchmarkLoggerDummy) Log(m *Message) error { PutMessage(m); return nil }

func (l *BenchmarkLoggerDummy) Close() error { return nil }

func (l *BenchmarkLoggerDummy) Name() string { return "dummy" }

func BenchmarkCopier64(b *testing.B) {
	benchmarkCopier(b, 1<<6)
}
func BenchmarkCopier128(b *testing.B) {
	benchmarkCopier(b, 1<<7)
}
func BenchmarkCopier256(b *testing.B) {
	benchmarkCopier(b, 1<<8)
}
func BenchmarkCopier512(b *testing.B) {
	benchmarkCopier(b, 1<<9)
}
func BenchmarkCopier1K(b *testing.B) {
	benchmarkCopier(b, 1<<10)
}
func BenchmarkCopier2K(b *testing.B) {
	benchmarkCopier(b, 1<<11)
}
func BenchmarkCopier4K(b *testing.B) {
	benchmarkCopier(b, 1<<12)
}
func BenchmarkCopier8K(b *testing.B) {
	benchmarkCopier(b, 1<<13)
}
func BenchmarkCopier16K(b *testing.B) {
	benchmarkCopier(b, 1<<14)
}
func BenchmarkCopier32K(b *testing.B) {
	benchmarkCopier(b, 1<<15)
}
func BenchmarkCopier64K(b *testing.B) {
	benchmarkCopier(b, 1<<16)
}
func BenchmarkCopier128K(b *testing.B) {
	benchmarkCopier(b, 1<<17)
}
func BenchmarkCopier256K(b *testing.B) {
	benchmarkCopier(b, 1<<18)
}

func piped(b *testing.B, iterations int, delay time.Duration, buf []byte) io.Reader {
	r, w, err := os.Pipe()
	if err != nil {
		b.Fatal(err)
		return nil
	}
	go func() {
		for i := 0; i < iterations; i++ {
			time.Sleep(delay)
			if n, err := w.Write(buf); err != nil || n != len(buf) {
				if err != nil {
					b.Fatal(err)
				}
				b.Fatal(fmt.Errorf("short write"))
			}
		}
		w.Close()
	}()
	return r
}

func benchmarkCopier(b *testing.B, length int) {
	b.StopTimer()
	buf := []byte{'A'}
	for len(buf) < length {
		buf = append(buf, buf...)
	}
	buf = append(buf[:length-1], []byte{'\n'}...)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		c := NewCopier(
			map[string]io.Reader{
				"buffer": piped(b, 10, time.Nanosecond, buf),
			},
			&BenchmarkLoggerDummy{})
		c.Run()
		c.Wait()
		c.Close()
	}
}
