package beam

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestSocketPair(t *testing.T) {
	a, b, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		a.Write([]byte("hello world!"))
		fmt.Printf("done writing. closing\n")
		a.Close()
		fmt.Printf("done closing\n")
	}()
	data, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("--> %s\n", data)
	fmt.Printf("still open: %v\n", a.Fd())
}

func TestSendUnixSocket(t *testing.T) {
	a1, a2, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	// defer a1.Close()
	// defer a2.Close()
	b1, b2, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	// defer b1.Close()
	// defer b2.Close()
	glueA, glueB, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	// defer glueA.Close()
	// defer glueB.Close()
	go func() {
		err := Send(b2, []byte("a"), glueB)
		if err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		err := Send(a2, []byte("b"), glueA)
		if err != nil {
			t.Fatal(err)
		}
	}()
	connAhdr, connA, err := Receive(a1)
	if err != nil {
		t.Fatal(err)
	}
	if string(connAhdr) != "b" {
		t.Fatalf("unexpected: %s", connAhdr)
	}
	connBhdr, connB, err := Receive(b1)
	if err != nil {
		t.Fatal(err)
	}
	if string(connBhdr) != "a" {
		t.Fatalf("unexpected: %s", connBhdr)
	}
	fmt.Printf("received both ends: %v <-> %v\n", connA.Fd(), connB.Fd())
	go func() {
		fmt.Printf("sending message on %v\n", connA.Fd())
		connA.Write([]byte("hello world"))
		connA.Sync()
		fmt.Printf("closing %v\n", connA.Fd())
		connA.Close()
	}()
	data, err := ioutil.ReadAll(connB)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("---> %s\n", data)
}
