package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

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

func (s *DockerSuite) TestCopier(c *check.C) {
	stdoutLine := "Line that thinks that it is log line from docker stdout"
	stderrLine := "Line that thinks that it is log line from docker stderr"
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for i := 0; i < 30; i++ {
		if _, err := stdout.WriteString(stdoutLine + "\n"); err != nil {
			c.Fatal(err)
		}
		if _, err := stderr.WriteString(stderrLine + "\n"); err != nil {
			c.Fatal(err)
		}
	}

	var jsonBuf bytes.Buffer

	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf)}

	nc := NewCopier(
		map[string]io.Reader{
			"stdout": &stdout,
			"stderr": &stderr,
		},
		jsonLog)
	nc.Run()
	wait := make(chan struct{})
	go func() {
		nc.Wait()
		close(wait)
	}()
	select {
	case <-time.After(1 * time.Second):
		c.Fatal("Copier failed to do its work in 1 second")
	case <-wait:
	}
	dec := json.NewDecoder(&jsonBuf)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			c.Fatal(err)
		}
		if msg.Source != "stdout" && msg.Source != "stderr" {
			c.Fatalf("Wrong Source: %q, should be %q or %q", msg.Source, "stdout", "stderr")
		}
		if msg.Source == "stdout" {
			if string(msg.Line) != stdoutLine {
				c.Fatalf("Wrong Line: %q, expected %q", msg.Line, stdoutLine)
			}
		}
		if msg.Source == "stderr" {
			if string(msg.Line) != stderrLine {
				c.Fatalf("Wrong Line: %q, expected %q", msg.Line, stderrLine)
			}
		}
	}
}

func (s *DockerSuite) TestCopierSlow(c *check.C) {
	stdoutLine := "Line that thinks that it is log line from docker stdout"
	var stdout bytes.Buffer
	for i := 0; i < 30; i++ {
		if _, err := stdout.WriteString(stdoutLine + "\n"); err != nil {
			c.Fatal(err)
		}
	}

	var jsonBuf bytes.Buffer
	//encoder := &encodeCloser{Encoder: json.NewEncoder(&jsonBuf)}
	jsonLog := &TestLoggerJSON{Encoder: json.NewEncoder(&jsonBuf), delay: 100 * time.Millisecond}

	nc := NewCopier(map[string]io.Reader{"stdout": &stdout}, jsonLog)
	nc.Run()
	wait := make(chan struct{})
	go func() {
		nc.Wait()
		close(wait)
	}()
	<-time.After(150 * time.Millisecond)
	nc.Close()
	select {
	case <-time.After(200 * time.Millisecond):
		c.Fatalf("failed to exit in time after the copier is closed")
	case <-wait:
	}
}

type BenchmarkLoggerDummy struct {
}

func (l *BenchmarkLoggerDummy) Log(m *Message) error { return nil }

func (l *BenchmarkLoggerDummy) Close() error { return nil }

func (l *BenchmarkLoggerDummy) Name() string { return "dummy" }

func (s *DockerSuite) BenchmarkCopier64(c *check.C) {
	benchmarkCopier(c, 1<<6)
}
func (s *DockerSuite) BenchmarkCopier128(c *check.C) {
	benchmarkCopier(c, 1<<7)
}
func (s *DockerSuite) BenchmarkCopier256(c *check.C) {
	benchmarkCopier(c, 1<<8)
}
func (s *DockerSuite) BenchmarkCopier512(c *check.C) {
	benchmarkCopier(c, 1<<9)
}
func (s *DockerSuite) BenchmarkCopier1K(c *check.C) {
	benchmarkCopier(c, 1<<10)
}
func (s *DockerSuite) BenchmarkCopier2K(c *check.C) {
	benchmarkCopier(c, 1<<11)
}
func (s *DockerSuite) BenchmarkCopier4K(c *check.C) {
	benchmarkCopier(c, 1<<12)
}
func (s *DockerSuite) BenchmarkCopier8K(c *check.C) {
	benchmarkCopier(c, 1<<13)
}
func (s *DockerSuite) BenchmarkCopier16K(c *check.C) {
	benchmarkCopier(c, 1<<14)
}
func (s *DockerSuite) BenchmarkCopier32K(c *check.C) {
	benchmarkCopier(c, 1<<15)
}
func (s *DockerSuite) BenchmarkCopier64K(c *check.C) {
	benchmarkCopier(c, 1<<16)
}
func (s *DockerSuite) BenchmarkCopier128K(c *check.C) {
	benchmarkCopier(c, 1<<17)
}
func (s *DockerSuite) BenchmarkCopier256K(c *check.C) {
	benchmarkCopier(c, 1<<18)
}

func piped(c *check.C, iterations int, delay time.Duration, buf []byte) io.Reader {
	r, w, err := os.Pipe()
	if err != nil {
		c.Fatal(err)
		return nil
	}
	go func() {
		for i := 0; i < iterations; i++ {
			time.Sleep(delay)
			if n, err := w.Write(buf); err != nil || n != len(buf) {
				if err != nil {
					c.Fatal(err)
				}
				c.Fatal(fmt.Errorf("short write"))
			}
		}
		w.Close()
	}()
	return r
}

func benchmarkCopier(c *check.C, length int) {
	c.StopTimer()
	buf := []byte{'A'}
	for len(buf) < length {
		buf = append(buf, buf...)
	}
	buf = append(buf[:length-1], []byte{'\n'}...)
	c.StartTimer()
	for i := 0; i < c.N; i++ {
		nc := NewCopier(
			map[string]io.Reader{
				"buffer": piped(c, 10, time.Nanosecond, buf),
			},
			&BenchmarkLoggerDummy{})
		nc.Run()
		nc.Wait()
		nc.Close()
	}
}
