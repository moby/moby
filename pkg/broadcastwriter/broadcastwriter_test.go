package broadcastwriter

import (
	"bytes"
	"errors"
	"strings"

	"testing"
)

type dummyWriter struct {
	buffer      bytes.Buffer
	failOnWrite bool
}

func equal(t *testing.T, dw *dummyWriter, value string) {
	if dw.String() != value {
		t.Errorf("Buffer contains %q", dw.String())
	}
}

func (dw *dummyWriter) Write(p []byte) (n int, err error) {
	if dw.failOnWrite {
		return 0, errors.New("Fake fail")
	}
	return dw.buffer.Write(p)
}

func (dw *dummyWriter) String() string {
	return dw.buffer.String()
}

func (dw *dummyWriter) Close() error {
	return nil
}

func TestBroadcastWriter(t *testing.T) {
	writer := New()

	// Test 1: Both bufferA and bufferB should contain "foo"
	bufferA := &dummyWriter{}
	writer.AddWriter(bufferA)
	bufferB := &dummyWriter{}
	writer.AddWriter(bufferB)
	writer.Write([]byte("foo"))

	equal(t, bufferA, "foo")

	equal(t, bufferB, "foo")

	// Test2: bufferA and bufferB should contain "foobar",
	// while bufferC should only contain "bar"
	bufferC := &dummyWriter{}
	writer.AddWriter(bufferC)
	writer.Write([]byte("bar"))

	equal(t, bufferA, "foobar")

	equal(t, bufferB, "foobar")

	equal(t, bufferC, "bar")

	// Test3: Test eviction on failure
	bufferA.failOnWrite = true
	writer.Write([]byte("fail"))
	equal(t, bufferA, "foobar")
	equal(t, bufferC, "barfail")

	// Even though we reset the flag, no more writes should go in there
	bufferA.failOnWrite = false
	writer.Write([]byte("test"))
	equal(t, bufferA, "foobar")
	equal(t, bufferC, "barfailtest")

	// Test4: Test eviction on multiple simultaneous failures
	bufferB.failOnWrite = true
	bufferC.failOnWrite = true
	bufferD := &dummyWriter{}
	writer.AddWriter(bufferD)
	writer.Write([]byte("yo"))
	writer.Write([]byte("ink"))
	if strings.Contains(bufferB.String(), "yoink") {
		t.Errorf("bufferB received write. contents: %q", bufferB)
	}
	if strings.Contains(bufferC.String(), "yoink") {
		t.Errorf("bufferC received write. contents: %q", bufferC)
	}
	if g, w := bufferD.String(), "yoink"; g != w {
		t.Errorf("bufferD = %q, want %q", g, w)
	}

	writer.Clean()
}

type devNullCloser int

func (d devNullCloser) Close() error {
	return nil
}

func (d devNullCloser) Write(buf []byte) (int, error) {
	return len(buf), nil
}

// This test checks for races. It is only useful when run with the race detector.
func TestRaceBroadcastWriter(t *testing.T) {
	writer := New()
	c := make(chan bool)
	go func() {
		writer.AddWriter(devNullCloser(0))
		c <- true
	}()
	writer.Write([]byte("hello"))
	<-c
}

func BenchmarkBroadcastWriter(b *testing.B) {
	writer := New()
	setUpWriter := func() {
		for i := 0; i < 100; i++ {
			writer.AddWriter(devNullCloser(0))
			writer.AddWriter(devNullCloser(0))
			writer.AddWriter(devNullCloser(0))
		}
	}
	testLine := "Line that thinks that it is log line from docker"
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		buf.Write([]byte(testLine + "\n"))
	}
	// line without eol
	buf.Write([]byte(testLine))
	testText := buf.Bytes()
	b.SetBytes(int64(5 * len(testText)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		setUpWriter()
		b.StartTimer()

		for j := 0; j < 5; j++ {
			if _, err := writer.Write(testText); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		writer.Clean()
		b.StartTimer()
	}
}
