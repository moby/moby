package beam

import (
	"io"
	"testing"
	"time"
)

func TestPipeClose(t *testing.T) {
	a, b := Pipe()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPipeSendReceive(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go a.Send([]byte("hello world!"), nil)
	data, s, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world!" {
		t.Fatalf("received incorrect data")
	}
	if s != nil {
		t.Fatalf("received incorrect stream")
	}
}

func TestPipeReceiveClose(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go a.Close()
	data, s, err := b.Receive()
	if err != io.EOF || data != nil || s != nil {
		t.Fatalf("incorrect receive after close: data=%#v s=%#v err=%#v", data, s, err)
	}
}
