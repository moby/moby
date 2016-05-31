package logger

import (
	"bytes"
	"encoding/json"
	"io"
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

func TestCopier(t *testing.T) {
	stdoutLine := "Line that thinks that it is log line from docker stdout"
	stderrLine := "Line that thinks that it is log line from docker stderr"
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
			if string(msg.Line) != stdoutLine {
				t.Fatalf("Wrong Line: %q, expected %q", msg.Line, stdoutLine)
			}
		}
		if msg.Source == "stderr" {
			if string(msg.Line) != stderrLine {
				t.Fatalf("Wrong Line: %q, expected %q", msg.Line, stderrLine)
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
		t.Fatalf("failed to exit in time after the copier is closed")
	case <-wait:
	}
}
