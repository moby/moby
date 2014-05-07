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

func TestUSocketPair(t *testing.T) {
	a, b, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}

	data := "hello world!"
	go func() {
		a.Write([]byte(data))
		a.Close()
	}()
	res := make([]byte, 1024)
	size, err := b.Read(res)
	if err != nil {
		t.Fatal(err)
	}
	if size != len(data) {
		t.Fatal("Unexpected size")
	}
	if string(res[0:size]) != data {
		t.Fatal("Unexpected data")
	}
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
		err := b2.Send([]byte("a"), glueB)
		if err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		err := a2.Send([]byte("b"), glueA)
		if err != nil {
			t.Fatal(err)
		}
	}()
	connAhdr, connA, err := a1.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(connAhdr) != "b" {
		t.Fatalf("unexpected: %s", connAhdr)
	}
	connBhdr, connB, err := b1.Receive()
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

// Ensure we get proper segmenting of messages
func TestSendSegmenting(t *testing.T) {
	a, b, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()

	extrafd1, extrafd2, err := SocketPair()
	if err != nil {
		t.Fatal(err)
	}
	extrafd2.Close()

	go func() {
		a.Send([]byte("message 1"), nil)
		a.Send([]byte("message 2"), extrafd1)
		a.Send([]byte("message 3"), nil)
	}()

	msg1, file1, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg1) != "message 1" {
		t.Fatal("unexpected msg1:", string(msg1))
	}
	if file1 != nil {
		t.Fatal("unexpectedly got file1")
	}

	msg2, file2, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg2) != "message 2" {
		t.Fatal("unexpected msg2:", string(msg2))
	}
	if file2 == nil {
		t.Fatal("didn't get file2")
	}
	file2.Close()

	msg3, file3, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg3) != "message 3" {
		t.Fatal("unexpected msg3:", string(msg3))
	}
	if file3 != nil {
		t.Fatal("unexpectedly got file3")
	}

}

// Test sending a zero byte message
func TestSendEmpty(t *testing.T) {
	a, b, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	go func() {
		a.Send([]byte{}, nil)
	}()

	msg, file, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) != 0 {
		t.Fatalf("unexpected non-empty message: %v", msg)
	}
	if file != nil {
		t.Fatal("unexpectedly got file")
	}

}

func makeLarge(size int) []byte {
	res := make([]byte, size)
	for i := range res {
		res[i] = byte(i % 255)
	}
	return res
}

func verifyLarge(data []byte, size int) bool {
	if len(data) != size {
		return false
	}
	for i := range data {
		if data[i] != byte(i%255) {
			return false
		}
	}
	return true
}

// Test sending a large message
func TestSendLarge(t *testing.T) {
	a, b, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	go func() {
		a.Send(makeLarge(100000), nil)
	}()

	msg, file, err := b.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if !verifyLarge(msg, 100000) {
		t.Fatalf("unexpected message (size %d)", len(msg))
	}
	if file != nil {
		t.Fatal("unexpectedly got file")
	}
}
