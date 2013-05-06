package docker

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/rcli"
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

func cmdWait(srv *Server, container *Container) error {
	stdout, stdoutPipe := io.Pipe()

	go func() {
		srv.CmdWait(nil, stdoutPipe, container.Id)
	}()

	if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
		return err
	}
	// Cleanup pipes
	return closeWrap(stdout, stdoutPipe)
}

// TestRunHostname checks that 'docker run -h' correctly sets a custom hostname
func TestRunHostname(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	stdin, _ := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	c := make(chan struct{})
	go func() {
		if err := srv.CmdRun(stdin, rcli.NewDockerLocalConn(stdoutPipe), "-h", "foobar", GetTestImage(runtime).Id, "hostname"); err != nil {
			t.Fatal(err)
		}
		close(c)
	}()
	cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if cmdOutput != "foobar\n" {
		t.Fatalf("'hostname' should display '%s', not '%s'", "foobar\n", cmdOutput)
	}

	setTimeout(t, "CmdRun timed out", 2*time.Second, func() {
		<-c
		cmdWait(srv, srv.runtime.List()[0])
	})

}

func TestRunExit(t *testing.T) {
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
		srv.CmdRun(stdin, rcli.NewDockerLocalConn(stdoutPipe), "-i", GetTestImage(runtime).Id, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	container := runtime.List()[0]

	// Closing /bin/cat stdin, expect it to exit
	p, err := container.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	// as the process exited, CmdRun must finish and unblock. Wait for it
	setTimeout(t, "Waiting for CmdRun timed out", 2*time.Second, func() {
		<-c1
		cmdWait(srv, container)
	})

	// Make sure that the client has been disconnected
	setTimeout(t, "The client should have been disconnected once the remote process exited.", 2*time.Second, func() {
		// Expecting pipe i/o error, just check that read does not block
		stdin.Read([]byte{})
	})

	// Cleanup pipes
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
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
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		srv.CmdRun(stdin, rcli.NewDockerLocalConn(stdoutPipe), "-i", GetTestImage(runtime).Id, "/bin/cat")
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

	// Client disconnect after run -i should cause stdin to be closed, which should
	// cause /bin/cat to exit.
	setTimeout(t, "Waiting for /bin/cat to exit timed out", 2*time.Second, func() {
		container := runtime.List()[0]
		container.Wait()
		if container.State.Running {
			t.Fatalf("/bin/cat is still running after closing stdin")
		}
	})
}

// Expected behaviour: the process dies when the client disconnects
func TestRunDisconnectTty(t *testing.T) {
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
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		srv.CmdRun(stdin, rcli.NewDockerLocalConn(stdoutPipe), "-i", "-t", GetTestImage(runtime).Id, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Waiting for the container to be started timed out", 2*time.Second, func() {
		for {
			// Client disconnect after run -i should keep stdin out in TTY mode
			l := runtime.List()
			if len(l) == 1 && l[0].State.Running {
				break
			}

			time.Sleep(10 * time.Millisecond)
		}
	})

	// Client disconnect after run -i should keep stdin out in TTY mode
	container := runtime.List()[0]

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// In tty mode, we expect the process to stay alive even after client's stdin closes.
	// Do not wait for run to finish

	// Give some time to monitor to do his thing
	container.WaitTimeout(500 * time.Millisecond)
	if !container.State.Running {
		t.Fatalf("/bin/cat should  still be running after closing stdin (tty mode)")
	}
}

// TestAttachStdin checks attaching to stdin without stdout and stderr.
// 'docker run -i -a stdin' should sends the client's stdin to the command,
// then detach from it and print the container id.
func TestRunAttachStdin(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)
	srv := &Server{runtime: runtime}

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	ch := make(chan struct{})
	go func() {
		srv.CmdRun(stdin, rcli.NewDockerLocalConn(stdoutPipe), "-i", "-a", "stdin", GetTestImage(runtime).Id, "sh", "-c", "echo hello; cat")
		close(ch)
	}()

	// Send input to the command, close stdin
	setTimeout(t, "Write timed out", 2*time.Second, func() {
		if _, err := stdinPipe.Write([]byte("hi there\n")); err != nil {
			t.Fatal(err)
		}
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})

	container := runtime.List()[0]

	// Check output
	cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if cmdOutput != container.ShortId()+"\n" {
		t.Fatalf("Wrong output: should be '%s', not '%s'\n", container.ShortId()+"\n", cmdOutput)
	}

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 2*time.Second, func() {
		<-ch
	})

	setTimeout(t, "Waiting for command to exit timed out", 2*time.Second, func() {
		container.Wait()
	})

	// Check logs
	if cmdLogs, err := container.ReadLog("stdout"); err != nil {
		t.Fatal(err)
	} else {
		if output, err := ioutil.ReadAll(cmdLogs); err != nil {
			t.Fatal(err)
		} else {
			expectedLog := "hello\nhi there\n"
			if string(output) != expectedLog {
				t.Fatalf("Unexpected logs: should be '%s', not '%s'\n", expectedLog, output)
			}
		}
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

	container, err := NewBuilder(runtime).Create(
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
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdAttach returns.
		srv.CmdAttach(stdin, rcli.NewDockerLocalConn(stdoutPipe), container.Id)
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
	container.Wait()
}
