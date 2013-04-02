package docker

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"testing"
)

func TestBufReader(t *testing.T) {
	reader, writer := io.Pipe()
	bufreader := newBufReader(reader)

	// Write everything down to a Pipe
	// Usually, a pipe should block but because of the buffered reader,
	// the writes will go through
	done := make(chan bool)
	go func() {
		writer.Write([]byte("hello world"))
		writer.Close()
		done <- true
	}()

	// Drain the reader *after* everything has been written, just to verify
	// it is indeed buffering
	<-done
	output, err := ioutil.ReadAll(bufreader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, []byte("hello world")) {
		t.Error(string(output))
	}
}

type dummyWriter struct {
	buffer      bytes.Buffer
	failOnWrite bool
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

func TestWriteBroadcaster(t *testing.T) {
	writer := newWriteBroadcaster()

	// Test 1: Both bufferA and bufferB should contain "foo"
	bufferA := &dummyWriter{}
	writer.AddWriter(bufferA)
	bufferB := &dummyWriter{}
	writer.AddWriter(bufferB)
	writer.Write([]byte("foo"))

	if bufferA.String() != "foo" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}

	if bufferB.String() != "foo" {
		t.Errorf("Buffer contains %v", bufferB.String())
	}

	// Test2: bufferA and bufferB should contain "foobar",
	// while bufferC should only contain "bar"
	bufferC := &dummyWriter{}
	writer.AddWriter(bufferC)
	writer.Write([]byte("bar"))

	if bufferA.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}

	if bufferB.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferB.String())
	}

	if bufferC.String() != "bar" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}

	// Test3: Test removal
	writer.RemoveWriter(bufferB)
	writer.Write([]byte("42"))
	if bufferA.String() != "foobar42" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}
	if bufferB.String() != "foobar" {
		t.Errorf("Buffer contains %v", bufferB.String())
	}
	if bufferC.String() != "bar42" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}

	// Test4: Test eviction on failure
	bufferA.failOnWrite = true
	writer.Write([]byte("fail"))
	if bufferA.String() != "foobar42" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}
	if bufferC.String() != "bar42fail" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}
	// Even though we reset the flag, no more writes should go in there
	bufferA.failOnWrite = false
	writer.Write([]byte("test"))
	if bufferA.String() != "foobar42" {
		t.Errorf("Buffer contains %v", bufferA.String())
	}
	if bufferC.String() != "bar42failtest" {
		t.Errorf("Buffer contains %v", bufferC.String())
	}

	writer.CloseWriters()
}

type devNullCloser int

func (d devNullCloser) Close() error {
	return nil
}

func (d devNullCloser) Write(buf []byte) (int, error) {
	return len(buf), nil
}

// This test checks for races. It is only useful when run with the race detector.
func TestRaceWriteBroadcaster(t *testing.T) {
	writer := newWriteBroadcaster()
	c := make(chan bool)
	go func() {
		writer.AddWriter(devNullCloser(0))
		c <- true
	}()
	writer.Write([]byte("hello"))
	<-c
}
