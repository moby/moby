package docker

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/term"
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

func setRaw(t *testing.T, c *daemon.Container) *term.State {
	pty, err := c.GetPtyMaster()
	if err != nil {
		t.Fatal(err)
	}
	state, err := term.MakeRaw(pty.Fd())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func unsetRaw(t *testing.T, c *daemon.Container, state *term.State) {
	pty, err := c.GetPtyMaster()
	if err != nil {
		t.Fatal(err)
	}
	term.RestoreTerminal(pty.Fd(), state)
}

func waitContainerStart(t *testing.T, timeout time.Duration) *daemon.Container {
	var container *daemon.Container

	setTimeout(t, "Waiting for the container to be started timed out", timeout, func() {
		for {
			l := globalDaemon.List()
			if len(l) == 1 && l[0].IsRunning() {
				container = l[0]
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	if container == nil {
		t.Fatal("An error occured while waiting for the container to start")
	}

	return container
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
	if <-c && msg != "" {
		t.Fatal(msg)
	}
}

func expectPipe(expected string, r io.Reader) error {
	o, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.Trim(o, " \r\n") != expected {
		return fmt.Errorf("Unexpected output. Expected [%s], received [%s]", expected, o)
	}
	return nil
}

func assertPipe(input, output string, r io.Reader, w io.Writer, count int) error {
	for i := 0; i < count; i++ {
		if _, err := w.Write([]byte(input)); err != nil {
			return err
		}
		if err := expectPipe(output, r); err != nil {
			return err
		}
	}
	return nil
}

// Expected behaviour: container gets deleted automatically after exit
func TestRunAutoRemove(t *testing.T) {
	t.Skip("Fixme. Skipping test for now, race condition")
	stdout, stdoutPipe := io.Pipe()

	cli := client.NewDockerCli(nil, stdoutPipe, ioutil.Discard, "", testDaemonProto, testDaemonAddr, nil)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("--rm", unitTestImageID, "hostname"); err != nil {
			t.Fatal(err)
		}
	}()

	var temporaryContainerID string
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		temporaryContainerID = cmdOutput
		if err := closeWrap(stdout, stdoutPipe); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "CmdRun timed out", 10*time.Second, func() {
		<-c
	})

	time.Sleep(500 * time.Millisecond)

	if len(globalDaemon.List()) > 0 {
		t.Fatalf("failed to remove container automatically: container %s still exists", temporaryContainerID)
	}
}
