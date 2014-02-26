package beam

import (
	"io"
	"sync"
	"testing"
	"time"
)

func TestFuncSend(t *testing.T) {
	var sentinel int
	f := Func(func(msg Message) error {
		if string(msg.Data) != "well hello..." {
			t.Fatalf("unexpected data: '%v'", msg.Data)
		}
		sentinel = 42
		return nil
	})
	local, remote := Pipe()
	defer local.Close()
	if err := f.Send(Message{Data: []byte("well hello..."), Stream: remote}); err != nil {
		t.Fatalf("send: %s", err)
	}
	if err := Splice(DevNull, local); err != nil {
		t.Fatalf("splice: %s", err)
	}
	if sentinel != 42 {
		t.Fatalf("sentinel was not set: was the handler func called?")
	}
}

func TestSendFunc(t *testing.T) {
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	mirror := Func(func(msg Message) error {
		if msg.Stream != nil {
			err := msg.Stream.Send(Message{Data: msg.Data})
			if err != nil {
				t.Fatalf("send: %s", err)
			}
		}
		return nil
	})
	var dumpCalled bool
	var wg sync.WaitGroup
	wg.Add(1)
	dump := Func(func(msg Message) error {
		defer wg.Done()
		if string(msg.Data) != "hello" {
			t.Fatalf("unexpected data: %s", string(msg.Data))
		}
		dumpCalled = true
		return nil
	})
	if err := mirror.Send(Message{Data: []byte("hello"), Stream: dump}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if !dumpCalled {
		t.Fatalf("function not called")
	}
}

func TestFuncReceive(t *testing.T) {
	f := Func(nil)
	_, err := f.Receive()
	if err != io.EOF {
		t.Fatalf("Func.Receive should return io.EOF")
	}
}
