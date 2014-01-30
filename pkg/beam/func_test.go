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
		return fmt.Errorf("this is a grave error")
	})
	if err := f.Send([]byte("well hello..."), nil); err == nil || err.Error() != "this is a grave error" {
		t.Fatalf("send: unexpected err='%v'", err)
	}
	if sentinel != 42 {
		t.Fatalf("sentinel was not set: was the handler func called?")
	}
}


func TestFuncReceive(t *testing.T) {
	f := Func(nil)
	_, _, err := f.Receive()
	if err == nil {
		t.Fatalf("Func should return an error on Receive")
	}
}
