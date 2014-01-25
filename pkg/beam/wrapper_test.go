package beam

import (
	"testing"
	"bytes"
)

func TestWrapBufferSend(t *testing.T) {
	buf := new(bytes.Buffer)
	wrapper := WrapIO(buf, 0)
	err := wrapper.Send([]byte("hello world!"), nil)
	if err != nil {
		t.Fatalf("send: %s", err)
	}
	if result := buf.String(); result != "hello world!" {
		t.Fatalf("buffer received incorrect data '%v'", result)
	}
}

func TestWrapBufferReceive(t *testing.T) {
	buf := bytes.NewBuffer([]byte("hi there"))
	wrapper := WrapIO(buf, 0)
	data, _, err := wrapper.Receive()
	if err != nil {
		t.Fatalf("receive: unexpected err=%v", err)
	}
	if string(data) != "hi there" {
		t.Fatalf("received wrong data from buffer: '%v'", data)
	}
}
