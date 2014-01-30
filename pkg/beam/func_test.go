package beam

import (
	"io"
	"testing"
)

func TestFuncSend(t *testing.T) {
	var sentinel int
	f := Func(func(data []byte, s Stream) error {
		if string(data) != "well hello..." {
			t.Fatalf("unexpected data: '%v'", data)
		}
		sentinel = 42
		return nil
	})
	local, remote := Pipe()
	defer local.Close()
	if err := f.Send([]byte("well hello..."), remote); err != nil {
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
	_, _, err := f.Receive()
	if err != io.EOF {
		t.Fatalf("Func.Receive should return io.EOF")
	}
}
