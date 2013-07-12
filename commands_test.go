package docker

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/utils"
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
	stdout, stdoutPipe := io.Pipe()

	cli := NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalRuntime)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-h", "foobar", unitTestImageID, "hostname"); err != nil {
			t.Fatal(err)
		}
	}()
	utils.Debugf("--")
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != "foobar\n" {
			t.Fatalf("'hostname' should display '%s', not '%s'", "foobar\n", cmdOutput)
		}
	})

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})

}


// TestAttachStdin checks attaching to stdin without stdout and stderr.
// 'docker run -i -a stdin' should sends the client's stdin to the command,
// then detach from it and print the container id.
func TestRunAttachStdin(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalRuntime)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		cli.CmdRun("-i", "-a", "stdin", unitTestImageID, "sh", "-c", "echo hello && cat")
	}()

	// Send input to the command, close stdin
	setTimeout(t, "Write timed out", 10*time.Second, func() {
		if _, err := stdinPipe.Write([]byte("hi there\n")); err != nil {
			t.Fatal(err)
		}
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})

	container := globalRuntime.List()[0]

	// Check output
	setTimeout(t, "Reading command output time out", 10*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != container.ShortID()+"\n" {
			t.Fatalf("Wrong output: should be '%s', not '%s'\n", container.ShortID()+"\n", cmdOutput)
		}
	})

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 5*time.Second, func() {
		// Unblock hijack end
		stdout.Read([]byte{})
		<-ch
	})

	setTimeout(t, "Waiting for command to exit timed out", 5*time.Second, func() {
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
