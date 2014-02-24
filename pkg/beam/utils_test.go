package beam

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

func TestCopy(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go func() {
		if err := a.Send(Message{Data: []byte("hello hello")}); err != nil {
			t.Fatalf("send: %s", err)
		}
		if err := a.Close(); err != nil {
			t.Fatalf("close: %s", err)
		}
	}()
	go func() {
		if err := Copy(b, a); err != nil {
			t.Fatalf("copy: %s", err)
		}
	}()
	if msg, err := b.Receive(); err != nil {
		t.Fatalf("receive: %s", err)
	} else if msg.Stream != nil {
		t.Fatalf("receive: wrong stream value %#v", msg.Stream)
	} else if string(msg.Data) != "hello hello" {
		t.Fatalf("receive: wrong data value %#v", msg.Data)
	}
}

func TestSplice(t *testing.T) {
	var wg sync.WaitGroup
	a, b := Pipe()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	wg.Add(3)
	go func() {
		defer wg.Done()
		sendExpect(a, "hello, I am A", "hello, I am B", t)
		a.Close()
	}()
	go func() {
		defer wg.Done()
		sendExpect(b, "hello, I am B", "hello, I am A", t)
		b.Close()
	}()
	go func() {
		defer wg.Done()
		if err := Splice(a, b); err != nil {
			t.Fatalf(": %s", err)
		}
	}()
	wg.Wait()
}

func TestDevNullReceive(t *testing.T) {
	msg, err := DevNull.Receive()
	if err != io.EOF {
		t.Fatalf("DevNull.Receive() should return io.EOF")
	}
	if msg.Data != nil && len(msg.Data) != 0 {
		t.Fatalf("DevNull.Receive() should not return data")
	}
	if msg.Stream != nil {
		t.Fatalf("DevNull.Receive() should not return a stream")
	}
}

func TestCopyLines(t *testing.T) {
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	input := getTestData(10)
	output := new(bytes.Buffer)
	err := Copy(WrapIO(output, 0), WrapIO(bytes.NewReader(input), 0))
	if err != nil {
		t.Fatalf("copy: %s", err)
	}
	if string(input) != output.String() {
		t.Fatalf("input != output: %v bytes vs %v", len(input), output.Len())
	}
}

func TestSpliceLines(t *testing.T) {
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	input := getTestData(10)
	output := new(bytes.Buffer)
	err := Splice(WrapIO(output, 0), WrapIO(bytes.NewReader(input), 0))
	if err != nil {
		t.Fatalf("splice: %s", err)
	}
	if string(input) != output.String() {
		t.Fatalf("input != output: %v bytes vs %v", len(input), output.Len())
	}
}

func TestSpliceClose(t *testing.T) {
	a1, a2 := Pipe()
	b1, b2 := Pipe()
	go func() {
		a1.Send(Message{Data: []byte("hello world!")})
		a1.Close()
	}()
	go Splice(a2, b1)
	if err := Copy(DevNull, b2); err != nil {
		t.Fatalf("copy: %s", err)
	}
}

func sendExpect(s Stream, send, expect string, t *testing.T) {
	if err := s.Send(Message{Data: []byte(send)}); err != nil {
		t.Fatalf("send: %s", err)
	}
	if msg, err := s.Receive(); err != io.EOF && err != nil {
		t.Fatalf("receive: %s", err)
	} else if string(msg.Data) != expect {
		t.Fatalf("expected: '%v'\nreceived '%v'", expect, string(msg.Data))
	}
}
