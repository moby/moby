package docker

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func closeWrap(args ...io.Closer) error {
	e := false
	ret := fmt.Errorf("Error closing elements")
	for _, c := range args {
		if err := c.Close(); err != nil {
			e = true
			ret = fmt.Errorf("%s\n%s", ret, err)
		}
	}
	if e {
		return ret
	}
	return nil
}

func setTimeout(t *testing.T, msg string, d time.Duration, f func(chan bool)) {
	c := make(chan bool)

	// Make sure we are not too long
	go func() {
		time.Sleep(d)
		c <- true
	}()
	go f(c)
	if timeout := <-c; timeout {
		t.Fatalf("Timeout: %s", msg)
	}
}

func assertPipe(input, output string, r io.Reader, w io.Writer, count int) error {
	for i := 0; i < count; i++ {
		if _, err := w.Write([]byte(input)); err != nil {
			return err
		}
		o, err := bufio.NewReader(r).ReadString('\n')
		if err != nil {
			return err
		}
		if strings.Trim(o, " \r\n") != output {
			return fmt.Errorf("Unexpected output. Expected [%s], received [%s]", output, o)
		}
	}
	return nil
}

// Test the behavior of a client disconnection.
// We expect a client disconnect to leave the stdin of the container open
// Therefore a process will keep his stdin open when a client disconnects
func TestReattachAfterDisconnect(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	// FIXME: low down the timeout (after #230)
	setTimeout(t, "TestReattachAfterDisconnect", 12*time.Second, func(timeout chan bool) {

		srv := &Server{runtime: runtime}

		stdin, stdinPipe := io.Pipe()
		stdout, stdoutPipe := io.Pipe()
		c1 := make(chan struct{})
		go func() {
			if err := srv.CmdRun(stdin, stdoutPipe, "-i", GetTestImage(runtime).Id, "/bin/cat"); err == nil {
				t.Fatal("CmdRun should generate a read/write on closed pipe error. No error found.")
			}
			close(c1)
		}()

		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}

		// Close pipes (simulate disconnect)
		if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
			t.Fatal(err)
		}

		container := runtime.containers.Back().Value.(*Container)

		// Recreate the pipes
		stdin, stdinPipe = io.Pipe()
		stdout, stdoutPipe = io.Pipe()

		// Attach to it
		c2 := make(chan struct{})
		go func() {
			if err := srv.CmdAttach(stdin, stdoutPipe, container.Id); err != nil {
				t.Fatal(err)
			}
			close(c2)
		}()

		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}

		// Close pipes
		if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
			t.Fatal(err)
		}

		// FIXME: when #230 will be finished, send SIGINT instead of SIGTERM
		//        we expect cat to stay alive so SIGTERM will have no effect
		//        and Stop will timeout
		if err := container.Stop(); err != nil {
			t.Fatal(err)
		}
		// Wait for run and attach to finish
		<-c1
		<-c2

		// Finished, no timeout
		timeout <- false
	})
}
