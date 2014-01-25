package beam

import (
	"io"
	"sync"
	"testing"
	"time"
)

func TestCopy(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go func() {
		if err := a.Send([]byte("hello hello"), nil); err != nil {
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
	if data, s, err := b.Receive(); err != nil {
		t.Fatalf("receive: %s", err)
	} else if s != nil {
		t.Fatalf("receive: wrong stream value %#v", s)
	} else if string(data) != "hello hello" {
		t.Fatalf("receive: wrong data value %#v", data)
	}
}

func TestSplice(t *testing.T) {
	var wg sync.WaitGroup
	a, b := Pipe()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
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

func sendExpect(s Stream, send, expect string, t *testing.T) {
	if err := s.Send([]byte(send), nil); err != nil {
		t.Fatalf("send: %s", err)
	}
	if data, _, err := s.Receive(); err != io.EOF && err != nil {
		t.Fatalf("receive: %s", err)
	} else if string(data) != expect {
		t.Fatalf("expected: '%v'\nreceived '%v'", expect, string(data))
	}
}
