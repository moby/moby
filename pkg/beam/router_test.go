package beam

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

type msg struct {
	payload    []byte
	attachment *os.File
}

func (m msg) String() string {
	return MsgDesc(m.payload, m.attachment)
}

type mockReceiver []msg

func (r *mockReceiver) Send(p []byte, a *os.File) error {
	(*r) = append((*r), msg{p, a})
	return nil
}

func TestSendNoSinkNoRoute(t *testing.T) {
	r := NewRouter(nil)
	if err := r.Send([]byte("hello"), nil); err == nil {
		t.Fatalf("error expected")
	}
	a, b, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	if err := r.Send([]byte("foo bar baz"), a); err == nil {
		t.Fatalf("error expected")
	}
}

func TestSendSinkNoRoute(t *testing.T) {
	var sink mockReceiver
	r := NewRouter(&sink)
	if err := r.Send([]byte("hello"), nil); err != nil {
		t.Fatal(err)
	}
	a, b, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	if err := r.Send([]byte("world"), a); err != nil {
		t.Fatal(err)
	}
	if len(sink) != 2 {
		t.Fatalf("%#v\n", sink)
	}
	if string(sink[0].payload) != "hello" {
		t.Fatalf("%#v\n", sink)
	}
	if sink[0].attachment != nil {
		t.Fatalf("%#v\n", sink)
	}
	if string(sink[1].payload) != "world" {
		t.Fatalf("%#v\n", sink)
	}
	if sink[1].attachment == nil || sink[1].attachment.Fd() > 42 || sink[1].attachment.Fd() < 0 {
		t.Fatalf("%v\n", sink)
	}
	var tasks sync.WaitGroup
	tasks.Add(2)
	go func() {
		defer tasks.Done()
		fmt.Printf("[%d] Reading from '%d'\n", os.Getpid(), sink[1].attachment.Fd())
		data, err := ioutil.ReadAll(sink[1].attachment)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "foo bar\n" {
			t.Fatalf("%v\n", string(data))
		}
	}()
	go func() {
		defer tasks.Done()
		fmt.Printf("[%d] writing to '%d'\n", os.Getpid(), a.Fd())
		if _, err := fmt.Fprintf(b, "foo bar\n"); err != nil {
			t.Fatal(err)
		}
		b.Close()
	}()
	tasks.Wait()
}
