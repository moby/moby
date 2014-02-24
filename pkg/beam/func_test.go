package beam

import (
	"io"
	"testing"
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

func TestFuncReceive(t *testing.T) {
	f := Func(nil)
	_, err := f.Receive()
	if err != io.EOF {
		t.Fatalf("Func.Receive should return io.EOF")
	}
}
