package npipe

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	clientMsg    = "Hi server!\n"
	serverMsg    = "Hi there, client!\n"
	fileTemplate = "62DA0493-99A1-4327-B5A8-6C4E4466C3FC.txt"
)

// TestBadDial tests that if you dial something other than a valid pipe path, that you get back a
// PipeError and that you don't accidently create a file on disk (since dial uses OpenFile)
func TestBadDial(t *testing.T) {
	fn := filepath.Join("C:\\", fileTemplate)
	ns := []string{fn, "http://www.google.com", "somethingbadhere"}
	for _, n := range ns {
		c, err := Dial(n)
		if _, ok := err.(PipeError); !ok {
			t.Errorf("Dialing '%s' did not result in correct error! Expected PipeError, got '%v'",
				n, err)
		}
		if c != nil {
			t.Errorf("Dialing '%s' returned non-nil connection", n)
		}
		if b, _ := exists(n); b {
			t.Errorf("Dialing '%s' incorrectly created file on disk", n)
		}
	}
}

// TestDialExistingFile tests that if you dial with the name of an existing file,
// that you don't accidentally open the file (since dial uses OpenFile)
func TestDialExistingFile(t *testing.T) {
	tempdir := os.TempDir()
	fn := filepath.Join(tempdir, fileTemplate)
	if f, err := os.Create(fn); err != nil {
		t.Fatalf("Unexpected error creating file '%s': '%v'", fn, err)
	} else {
		// we don't actually need to write to the file, just need it to exist
		f.Close()
		defer os.Remove(fn)
	}
	c, err := Dial(fn)
	if _, ok := err.(PipeError); !ok {
		t.Errorf("Dialing '%s' did not result in error! Expected PipeError, got '%v'", fn, err)
	}
	if c != nil {
		t.Errorf("Dialing '%s' returned non-nil connection", fn)
	}
}

// TestBadListen tests that if you listen on a bad address, that we get back a PipeError
func TestBadListen(t *testing.T) {
	addrs := []string{"not a valid pipe address", `\\127.0.0.1\pipe\TestBadListen`}
	for _, address := range addrs {
		ln, err := Listen(address)
		if _, ok := err.(PipeError); !ok {
			t.Errorf("Listening on '%s' did not result in correct error! Expected PipeError, got '%v'",
				address, err)
		}
		if ln != nil {
			t.Errorf("Listening on '%s' returned non-nil listener.", address)
		}
	}
}

// TestDoubleListen makes sure we can't listen to the same address twice.
func TestDoubleListen(t *testing.T) {
	address := `\\.\pipe\TestDoubleListen`
	ln1, err := Listen(address)
	if err != nil {
		t.Fatalf("Listen(%q): %v", address, err)
	}
	defer ln1.Close()

	ln2, err := Listen(address)
	if err == nil {
		ln2.Close()
		t.Fatalf("second Listen on %q succeeded.", address)
	}
}

// TestPipeConnected tests whether we correctly handle clients connecting
// and then closing the connection between creating and connecting the
// pipe on the server side.
func TestPipeConnected(t *testing.T) {
	address := `\\.\pipe\TestPipeConnected`
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Listen(%q): %v", address, err)
	}
	defer ln.Close()

	// Create a client connection and close it immediately.
	clientConn, err := Dial(address)
	if err != nil {
		t.Fatalf("Error from dial: %v", err)
	}
	clientConn.Close()

	content := "test"
	go func() {
		// Now create a real connection and send some data.
		clientConn, err := Dial(address)
		if err != nil {
			t.Fatalf("Error from dial: %v", err)
		}
		if _, err := clientConn.Write([]byte(content)); err != nil {
			t.Fatalf("Error writing to pipe: %v", err)
		}
		clientConn.Close()
	}()

	serverConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Error from accept: %v", err)
	}
	result, err := ioutil.ReadAll(serverConn)
	if err != nil {
		t.Fatalf("Error from ReadAll: %v", err)
	}
	if string(result) != content {
		t.Fatalf("Got %s, expected: %s", string(result), content)
	}
	serverConn.Close()
}

// TestListenCloseListen tests whether Close() actually closes a named pipe properly.
func TestListenCloseListen(t *testing.T) {
	address := `\\.\pipe\TestListenCloseListen`
	ln1, err := Listen(address)
	if err != nil {
		t.Fatalf("Listen(%q): %v", address, err)
	}
	ln1.Close()

	ln2, err := Listen(address)
	if err != nil {
		t.Fatalf("second Listen on %q failed.", address)
	}
	ln2.Close()
}

// TestCloseFileHandles tests that all PipeListener handles are actualy closed after
// calling Close()
func TestCloseFileHandles(t *testing.T) {
	address := `\\.\pipe\TestCloseFileHandles`
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Error listening on %q: %v", address, err)
	}
	defer ln.Close()
	server := rpc.NewServer()
	service := &RPCService{}
	server.Register(service)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Ignore errors produced by a closed listener.
				if err != ErrClosed {
					t.Errorf("ln.Accept(): %v", err.Error())
				}
				break
			}
			go server.ServeConn(conn)
		}
	}()
	conn, err := Dial(address)
	if err != nil {
		t.Fatalf("Error dialing %q: %v", address, err)
	}
	client := rpc.NewClient(conn)
	defer client.Close()
	req := "dummy"
	resp := ""
	if err = client.Call("RPCService.GetResponse", req, &resp); err != nil {
		t.Fatalf("Error calling RPCService.GetResponse: %v", err)
	}
	if req != resp {
		t.Fatalf("Unexpected result (expected: %q, got: %q)", req, resp)
	}
	ln.Close()

	if ln.acceptHandle != 0 {
		t.Fatalf("Failed to close acceptHandle")
	}
	if ln.acceptOverlapped.HEvent != 0 {
		t.Fatalf("Failed to close acceptOverlapped handle")
	}
}

// TestCancelListen tests whether Accept() can be cancelled by closing the listener.
func TestCancelAccept(t *testing.T) {
	address := `\\.\pipe\TestCancelListener`
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Listen(%q): %v", address, err)
	}
	cancelled := make(chan struct{}, 0)
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			t.Fatalf("Unexpected incoming connection: %v", conn)
			conn.Close()
		}
		cancelled <- struct{}{}
	}()
	// Close listener after 20ms. This should give the go routine enough time to be actually
	// waiting for incoming connections inside ln.Accept().
	time.AfterFunc(20*time.Millisecond, func() {
		if err := ln.Close(); err != nil {
			t.Fatalf("Error closing listener: %v", err)
		}
	})
	// Any Close() should abort the ln.Accept() call within 100ms.
	// We fail with a timeout otherwise, to avoid blocking forever on a failing test.
	timeout := time.After(100 * time.Millisecond)
	select {
	case <-cancelled:
		// This is what should happen.
	case <-timeout:
		t.Fatal("Timeout trying to cancel accept.")
	}
}

// Test that PipeConn's read deadline works correctly
func TestReadDeadline(t *testing.T) {
	address := `\\.\pipe\TestReadDeadline`
	var wg sync.WaitGroup
	wg.Add(1)

	go listenAndWait(address, wg, t)
	defer wg.Done()

	c, err := Dial(address)
	if err != nil {
		t.Fatalf("Error dialing into pipe: %v", err)
	}
	if c == nil {
		t.Fatal("Unexpected nil connection from Dial")
	}
	defer c.Close()
	deadline := time.Now().Add(time.Millisecond * 50)
	c.SetReadDeadline(deadline)
	msg, err := bufio.NewReader(c).ReadString('\n')
	end := time.Now()
	if msg != "" {
		t.Errorf("Pipe read timeout returned a non-empty message: %s", msg)
	}
	if err == nil {
		t.Error("Pipe read timeout returned nil error")
	} else {
		pe, ok := err.(PipeError)
		if !ok {
			t.Errorf("Got wrong error returned, expected PipeError, got '%t'", err)
		}
		if !pe.Timeout() {
			t.Error("Pipe read timeout didn't return an error indicating the timeout")
		}
	}
	checkDeadline(deadline, end, t)
}

// listenAndWait simply sets up a pipe listener that does nothing and closes after the waitgroup
// is done.
func listenAndWait(address string, wg sync.WaitGroup, t *testing.T) {
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Error starting to listen on pipe: %v", err)
	}
	if ln == nil {
		t.Fatal("Got unexpected nil listener")
	}
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Error accepting connection: %v", err)
	}
	if conn == nil {
		t.Fatal("Got unexpected nil connection")
	}
	defer conn.Close()
	// don't read or write anything
	wg.Wait()
}

// TestWriteDeadline tests that PipeConn's write deadline works correctly
func TestWriteDeadline(t *testing.T) {
	address := `\\.\pipe\TestWriteDeadline`
	var wg sync.WaitGroup
	wg.Add(1)

	go listenAndWait(address, wg, t)
	defer wg.Done()
	c, err := Dial(address)
	if err != nil {
		t.Fatalf("Error dialing into pipe: %v", err)
	}
	if c == nil {
		t.Fatal("Unexpected nil connection from Dial")
	}

	// windows pipes have a buffer, so even if we don't read from the pipe,
	// the write may succeed anyway, so we have to write a whole bunch to
	// test the time out
	deadline := time.Now().Add(time.Millisecond * 50)
	c.SetWriteDeadline(deadline)
	buffer := make([]byte, 1<<16)
	if _, err = io.ReadFull(rand.Reader, buffer); err != nil {
		t.Fatalf("Couldn't generate random buffer: %v", err)
	}
	_, err = c.Write(buffer)

	end := time.Now()

	if err == nil {
		t.Error("Pipe write timeout returned nil error")
	} else {
		pe, ok := err.(PipeError)
		if !ok {
			t.Errorf("Got wrong error returned, expected PipeError, got '%t'", err)
		}
		if !pe.Timeout() {
			t.Error("Pipe write timeout didn't return an error indicating the timeout")
		}
	}
	checkDeadline(deadline, end, t)
}

// TestDialTimeout tests that the DialTimeout function will actually timeout correctly
func TestDialTimeout(t *testing.T) {
	timeout := time.Millisecond * 150
	deadline := time.Now().Add(timeout)
	c, err := DialTimeout(`\\.\pipe\TestDialTimeout`, timeout)
	end := time.Now()
	if c != nil {
		t.Errorf("DialTimeout returned non-nil connection: %v", c)
	}
	if err == nil {
		t.Error("DialTimeout returned nil error after timeout")
	} else {
		pe, ok := err.(PipeError)
		if !ok {
			t.Errorf("Got wrong error returned, expected PipeError, got '%t'", err)
		}
		if !pe.Timeout() {
			t.Error("Dial timeout didn't return an error indicating the timeout")
		}
	}
	checkDeadline(deadline, end, t)
}

// TestDialNoTimeout tests that the DialTimeout function will properly wait for the pipe and
// connect when it is available
func TestDialNoTimeout(t *testing.T) {
	timeout := time.Millisecond * 500
	address := `\\.\pipe\TestDialNoTimeout`
	go func() {
		<-time.After(50 * time.Millisecond)
		listenAndClose(address, t)
	}()

	deadline := time.Now().Add(timeout)
	c, err := DialTimeout(address, timeout)
	end := time.Now()

	if c == nil {
		t.Error("DialTimeout returned unexpected nil connection")
	}
	if err != nil {
		t.Error("DialTimeout returned unexpected non-nil error: ", err)
	}
	if end.After(deadline) {
		t.Fatalf("Ended %v after deadline", end.Sub(deadline))
	}
}

// TestDial tests that you can dial before a pipe is available,
// and that it'll pick up the pipe once it's ready
func TestDial(t *testing.T) {
	address := `\\.\pipe\TestDial`
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		conn, err := Dial(address)
		if err != nil {
			t.Fatalf("Got unexpected error from Dial: %v", err)
		}
		if conn == nil {
			t.Fatal("Got unexpected nil connection from Dial")
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("Got unexpected error from conection.Close(): %v", err)
		}
	}()

	wg.Wait()
	<-time.After(50 * time.Millisecond)
	listenAndClose(address, t)
}

type RPCService struct{}

func (s *RPCService) GetResponse(request string, response *string) error {
	*response = request
	return nil
}

// TestGoRPC tests that you can run go RPC over the pipe,
// and that overlapping bi-directional communication is working
// (write while a blocking read is in progress).
func TestGoRPC(t *testing.T) {
	address := `\\.\pipe\TestRPC`
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Error listening on %q: %v", address, err)
	}
	defer ln.Close()
	waitExit := make(chan bool)
	defer func() {
		ln.Close()
		<-waitExit
	}()
	server := rpc.NewServer()
	service := &RPCService{}
	server.Register(service)
	go server.Accept(ln)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Ignore errors produced by a closed listener.
				if err != ErrClosed {
					t.Errorf("ln.Accept(): %v", err.Error())
				}
				break
			}
			go server.ServeConn(conn)
		}
		waitExit <- true
	}()
	var conn *PipeConn
	conn, err = Dial(address)
	if err != nil {
		t.Fatalf("Error dialing %q: %v", address, err)
	}
	client := rpc.NewClient(conn)
	defer client.Close()
	req := "dummy"
	resp := ""
	if err = client.Call("RPCService.GetResponse", req, &resp); err != nil {
		t.Fatalf("Error calling RPCService.GetResponse: %v", err)
	}
	if req != resp {
		t.Fatalf("Unexpected result (expected: %q, got: %q)", req, resp)
	}
}

// listenAndClose is a helper method to just listen on a pipe and close as soon as someone connects.
func listenAndClose(address string, t *testing.T) {
	ln, err := Listen(address)
	if err != nil {
		t.Fatalf("Got unexpected error from Listen: %v", err)
	}
	if ln == nil {
		t.Fatal("Got unexpected nil listener from Listen")
	}
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Got unexpected error from Accept: %v", err)
	}
	if conn == nil {
		t.Fatal("Got unexpected nil connection from Accept")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Got unexpected error from conection.Close(): %v", err)
	}
}

// TestCommonUseCase is a full run-through of the most common use case, where you create a listener
// and then dial into it with several clients in succession
func TestCommonUseCase(t *testing.T) {
	addrs := []string{`\\.\pipe\TestCommonUseCase`, `\\127.0.0.1\pipe\TestCommonUseCase`}
	// always listen on the . version, since IP won't work for listening
	ln, err := Listen(addrs[0])
	if err != nil {
		t.Fatalf("Listen(%q) failed: %v", addrs[0], err)
	}
	defer ln.Close()

	for _, address := range addrs {
		convos := 5
		clients := 10

		done := make(chan bool)
		quit := make(chan bool)

		go aggregateDones(done, quit, clients)

		for x := 0; x < clients; x++ {
			go startClient(address, done, convos, t)
		}

		go startServer(ln, convos, t)

		select {
		case <-quit:
		case <-time.After(time.Second):
			t.Fatal("Failed to receive quit message after a reasonable timeout")
		}
	}
}

// aggregateDones simply aggregates messages from the done channel
// until it sees total, and then sends a message on the quit channel
func aggregateDones(done, quit chan bool, total int) {
	dones := 0
	for dones < total {
		<-done
		dones++
	}
	quit <- true
}

// startServer accepts connections and spawns goroutines to handle them
func startServer(ln *PipeListener, iter int, t *testing.T) {
	for {
		conn, err := ln.Accept()
		if err == ErrClosed {
			return
		}
		if err != nil {
			t.Fatalf("Error accepting connection: %v", err)
		}
		go handleConnection(conn, iter, t)
	}
}

// handleConnection is the goroutine that handles connections on the server side
// it expects to read a message and then write a message, convos times, before exiting.
func handleConnection(conn net.Conn, convos int, t *testing.T) {
	r := bufio.NewReader(conn)
	for x := 0; x < convos; x++ {
		msg, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("Error reading from server connection: %v", err)
		}
		if msg != clientMsg {
			t.Fatalf("Read incorrect message from client. Expected '%s', got '%s'", clientMsg, msg)
		}

		if _, err := fmt.Fprint(conn, serverMsg); err != nil {
			t.Fatalf("Error on server writing to pipe: %v", err)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Error closing server side of connection: %v", err)
	}
}

// startClient waits on a pipe at the given address. It expects to write a message and then
// read a message from the pipe, convos times, and then sends a message on the done
// channel
func startClient(address string, done chan bool, convos int, t *testing.T) {
	c := make(chan *PipeConn)
	go asyncdial(address, c, t)

	var conn *PipeConn
	select {
	case conn = <-c:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Client timed out waiting for dial to resolve")
	}
	r := bufio.NewReader(conn)
	for x := 0; x < convos; x++ {
		if _, err := fmt.Fprint(conn, clientMsg); err != nil {
			t.Fatalf("Error on client writing to pipe: %v", err)
		}

		msg, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("Error reading from client connection: %v", err)
		}
		if msg != serverMsg {
			t.Fatalf("Read incorrect message from server. Expected '%s', got '%s'", serverMsg, msg)
		}
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Error closing client side of pipe %v", err)
	}
	done <- true
}

// asyncdial is a helper that dials and returns the connection on the given channel.
// this is useful for being able to give dial a timeout
func asyncdial(address string, c chan *PipeConn, t *testing.T) {
	conn, err := Dial(address)
	if err != nil {
		t.Fatalf("Error from dial: %v", err)
	}
	c <- conn
}

// exists is a simple helper function to detect if a file exists on disk
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func checkDeadline(deadline, end time.Time, t *testing.T) {
	if end.Before(deadline) {
		t.Fatalf("Ended %v before deadline", deadline.Sub(end))
	}
	diff := end.Sub(deadline)

	// we need a huge fudge factor here because Windows has really poor
	// resolution for timeouts, and in practice, the timeout can be 400ms or
	// more after the expected timeout.
	if diff > 500*time.Millisecond {
		t.Fatalf("Ended significantly (%v) after deadline", diff)
	}
}
