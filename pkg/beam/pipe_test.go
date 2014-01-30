package beam

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
	"bytes"
)

func TestPipeClose(t *testing.T) {
	a, b := Pipe()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Send([]byte("foo"), nil); err == nil {
		t.Fatalf("sending on a closed pipe should return an error")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Send([]byte("foo"), nil); err == nil {
		t.Fatalf("sending on a closed pipe should return an error")
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


func TestPipeSendThenClose(t *testing.T) {
	a, b := Pipe()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	go func() {
		a.Send([]byte("hello world"), nil)
		a.Close()
	}()
	if data, _, _:= b.Receive(); string(data) != "hello world" {
		t.Fatalf("receive: unexpected data '%s'", data)
	}
	if data, s, err := b.Receive(); err != io.EOF || data != nil || s != nil {
		t.Fatalf("incorrect receive after close: data=%#v s=%#v err=%#v", data, s, err)
	}
}

func TestSendBuf(t *testing.T) {
	a, b := Pipe()
	defer a.Close()
	defer b.Close()
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	buf := WrapIO(strings.NewReader("hello there"), 0)
	go func() {
		if err := a.Send([]byte("stdout"), buf); err != nil {
			t.Fatalf("send: %s", err)
		}
	}()
	if data, s, err := b.Receive(); err != nil {
		t.Fatalf("receive: %s", err)
	} else if string(data) != "stdout" {
		t.Fatalf("receive: unexpected data '%s'", data)
	} else if s == nil {
		t.Fatalf("receive: expected valid stream")
	} else {
		msg, _, err := s.Receive()
		if err != nil {
			t.Fatalf("receive: %s", err)
		}
		if string(msg) != "hello there" {
			t.Fatalf("receive: unexpected data '%s'", msg)
		}
		Splice(s, DevNull)
	}
}

func getTestData(lines int) []byte {
	buf := new(bytes.Buffer)
	for i:=0; i < lines; i++ {
		fmt.Fprintf(buf, "this is line %d\n", i)
	}
	return buf.Bytes()
}

func TestSendLines(t *testing.T) {
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
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
	timer := time.AfterFunc(1 * time.Second, func() { t.Fatalf("timeout") })
	defer timer.Stop()
	testData := getTestData(10)
	a, b := Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		x, y := Pipe()
		defer x.Close()
		if err := a.Send([]byte("stdout"), y); err != nil {
			y.Close()
			t.Fatalf("send: %s", err)
		}
		if _, err := io.Copy(NewWriter(x), bytes.NewReader(testData)); err != nil {
			t.Fatalf("copy: %s", err)
		}
	}()
	header, stdout, err := b.Receive()
	if err != nil {
		t.Fatalf("receive: %s", err)
	}
	if string(header) != "stdout" {
		t.Fatalf("unexpected data '%s'", header)
	}
	if stdout == nil {
		t.Fatalf("receive: expected valid stream")
	}
	outputData, err := ioutil.ReadAll(NewReader(stdout))
	if err != nil {
		t.Fatalf("readall: %s", err)
	}
	if string(outputData) != string(testData) {
		t.Fatalf("output doesn't match input (%d bytes vs %d)", len(outputData), len(testData))
	}
}
