package beam

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func TestWrapBufferSend(t *testing.T) {
	buf := new(bytes.Buffer)
	wrapper := WrapIO(buf, 0)
	err := wrapper.Send(Message{Data: []byte("hello world!")})
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
	msg, err := wrapper.Receive()
	if err != nil {
		t.Fatalf("receive: unexpected err=%v", err)
	}
	if string(msg.Data) != "hi there" {
		t.Fatalf("received wrong data from buffer: '%v'", msg.Data)
	}
}

func TestWrapSendNoWrite(t *testing.T) {
	r := bytes.NewReader([]byte("foobar"))
	err := WrapIO(r, 0).Send(Message{Data: []byte("something")})
	if err != nil {
		t.Fatalf("IOWrapper.Send must silently discard when Write is not implemented (err=%s)", err)
	}
}

func TestWrapReceiveNoRead(t *testing.T) {
	w := bufio.NewWriter(ioutil.Discard)
	_, err := WrapIO(w, 0).Receive()
	if err != io.EOF {
		t.Fatalf("IOWrapper.Receive must return EOF when Read is not implemented (err=%s)", err)
	}
}
