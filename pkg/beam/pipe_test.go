package beam

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestPipeClose(t *testing.T) {
	a, b := Pipe()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Send(Message{Data: []byte("foo")}); err == nil {
		t.Fatalf("sending on a closed pipe should return an error")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Send(Message{Data: []byte("foo")}); err == nil {
		t.Fatalf("sending on a closed pipe should return an error")
	}
}

func TestPipeSendReceive(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go a.Send(Message{Data: []byte("hello world!")})
	msg, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg.Data) != "hello world!" {
		t.Fatalf("received incorrect data")
	}
	if msg.Stream != nil {
		t.Fatalf("received incorrect stream")
	}
}

func TestPipeReceiveClose(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go a.Close()
	msg, err := b.Receive()
	if err != io.EOF || msg.Data != nil || msg.Stream != nil {
		t.Fatalf("incorrect receive after close: data=%#v s=%#v err=%#v", msg.Data, msg.Stream, err)
	}
}

func TestPipeSendThenClose(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go func() {
		a.Send(Message{Data: []byte("hello world")})
		a.Close()
	}()
	if msg, _ := b.Receive(); string(msg.Data) != "hello world" {
		t.Fatalf("receive: unexpected data '%s'", msg.Data)
	}
	if msg, err := b.Receive(); err != io.EOF || msg.Data != nil || msg.Stream != nil {
		t.Fatalf("incorrect receive after close: data=%#v s=%#v err=%#v", msg.Data, msg.Stream, err)
	}
}

func TestSendBuf(t *testing.T) {
	a, b := Pipe()
	defer a.Close()
	defer b.Close()
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	buf := WrapIO(strings.NewReader("hello there"), 0)
	go func() {
		if err := a.Send(Message{Data: []byte("stdout"), Stream: buf}); err != nil {
			t.Fatalf("send: %s", err)
		}
	}()
	if msg, err := b.Receive(); err != nil {
		t.Fatalf("receive: %s", err)
	} else if string(msg.Data) != "stdout" {
		t.Fatalf("receive: unexpected data '%s'", msg.Data)
	} else if msg.Stream == nil {
		t.Fatalf("receive: expected valid stream")
	} else {
		subMsg, err := msg.Stream.Receive()
		if err != nil {
			t.Fatalf("receive: %s", err)
		}
		if string(subMsg.Data) != "hello there" {
			t.Fatalf("receive: unexpected data '%s'", subMsg.Data)
		}
		Splice(msg.Stream, DevNull)
	}
}

func getTestData(lines int) []byte {
	buf := new(bytes.Buffer)
	for i := 0; i < lines; i++ {
		fmt.Fprintf(buf, "this is line %d\n", i)
	}
	return buf.Bytes()
}

func TestSendLines(t *testing.T) {
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	input := getTestData(10)
	a, b := Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		if _, err := io.Copy(NewWriter(a), bytes.NewReader(input)); err != nil {
			t.Fatalf("copy: %s", err)
		}
		a.Close()
	}()
	output, err := ioutil.ReadAll(NewReader(b))
	if err != nil {
		t.Fatalf("readall: %s", err)
	}
	if string(input) != string(output) {
		t.Fatalf("input != output: %s bytes vs %d", len(input), len(output))
	}
}

func TestSendPipe(t *testing.T) {
	timer := time.AfterFunc(1*time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	testData := getTestData(10)
	a, b := Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		x, y := Pipe()
		defer x.Close()
		if err := a.Send(Message{Data: []byte("stdout"), Stream: y}); err != nil {
			y.Close()
			t.Fatalf("send: %s", err)
		}
		if _, err := io.Copy(NewWriter(x), bytes.NewReader(testData)); err != nil {
			t.Fatalf("copy: %s", err)
		}
	}()
	stdout, err := b.Receive()
	if err != nil {
		t.Fatalf("receive: %s", err)
	}
	if string(stdout.Data) != "stdout" {
		t.Fatalf("unexpected data '%s'", stdout.Data)
	}
	if stdout.Stream == nil {
		t.Fatalf("receive: expected valid stream")
	}
	outputData, err := ioutil.ReadAll(NewReader(stdout.Stream))
	if err != nil {
		t.Fatalf("readall: %s", err)
	}
	if string(outputData) != string(testData) {
		t.Fatalf("output doesn't match input (%d bytes vs %d)", len(outputData), len(testData))
	}
}
