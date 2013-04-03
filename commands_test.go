package docker

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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

func setTimeout(t *testing.T, msg string, d time.Duration, f func()) {
	c := make(chan bool)

	// Make sure we are not too long
	go func() {
		time.Sleep(d)
		c <- true
	}()
	go func() {
		f()
		c <- false
	}()
	if <-c {
		t.Fatal(msg)
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

// TestRunHostname checks that 'docker run -h' correctly sets a custom hostname
func TestRunHostname(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	var stdin, stdout bytes.Buffer
	setTimeout(t, "CmdRun timed out", 2*time.Second, func() {
		if err := srv.CmdRun(ioutil.NopCloser(&stdin), &nopWriteCloser{&stdout}, "-h", "foobar", GetTestImage(runtime).Id, "hostname"); err != nil {
			t.Fatal(err)
		}
	})
	if output := string(stdout.Bytes()); output != "foobar\n" {
		t.Fatalf("'hostname' should display '%s', not '%s'", "foobar\n", output)
	}
}

// Expected behaviour: the process dies when the client disconnects
func TestRunDisconnect(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()
	c1 := make(chan struct{})
	go func() {
		if err := srv.CmdRun(stdin, stdoutPipe, "-i", GetTestImage(runtime).Id, "/bin/cat"); err != nil {
			t.Fatal(err)
		}
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// as the pipes are close, we expect the process to die,
	// therefore CmdRun to unblock. Wait for CmdRun
	setTimeout(t, "Waiting for CmdRun timed out", 2*time.Second, func() {
		<-c1
	})

	// Check the status of the container
	container := runtime.containers.Back().Value.(*Container)
	if container.State.Running {
		t.Fatalf("/bin/cat is still running after closing stdin")
	}
}

// Expected behaviour, the process stays alive when the client disconnects
func TestAttachDisconnect(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	container, err := runtime.Create(
		&Config{
			Image:     GetTestImage(runtime).Id,
			Memory:    33554432,
			Cmd:       []string{"/bin/cat"},
			OpenStdin: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Destroy(container)

	// Start the process
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		if err := srv.CmdAttach(stdin, stdoutPipe, container.Id); err != nil {
			t.Fatal(err)
		}
		close(c1)
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})
	// Close pipes (client disconnects)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	setTimeout(t, "Waiting for CmdAttach timed out", 2*time.Second, func() {
		<-c1
	})
	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	err = container.WaitTimeout(500 * time.Millisecond)
	if err == nil || !container.State.Running {
		t.Fatalf("/bin/cat is not running after closing stdin")
	}

	// Try to avoid the timeoout in destroy. Best effort, don't check error
	cStdin, _ := container.StdinPipe()
	cStdin.Close()
}
