package beam

import (
	"bufio"
	"io"
	"io/ioutil"
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

func TestWrapSendNoWrite(t *testing.T) {
	r := bytes.NewReader([]byte("foobar"))
	err := WrapIO(r, 0).Send([]byte("something"), nil)
	if err != nil {
		t.Fatalf("IOWrapper.Send must silently discard when Write is not implemented (err=%s)", err)
	}
}

func TestWrapReceiveNoRead(t *testing.T) {
	w := bufio.NewWriter(ioutil.Discard)
	_, _, err := WrapIO(w, 0).Receive()
	if err != io.EOF {
		t.Fatalf("IOWrapper.Receive must return EOF when Read is not implemented (err=%s)", err)
	}
}
