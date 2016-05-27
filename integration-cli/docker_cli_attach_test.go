package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

const attachWait = 5 * time.Second

func (s *DockerSuite) TestAttachMultipleAndRestart(c *check.C) {
	testRequires(c, DaemonIsLinux)

	endGroup := &sync.WaitGroup{}
	startGroup := &sync.WaitGroup{}
	endGroup.Add(3)
	startGroup.Add(3)

	err := waitForContainer("attacher", "-d", "busybox", "/bin/sh", "-c", "while true; do sleep 1; echo hello; done")
	c.Assert(err, check.IsNil)

	startDone := make(chan struct{})
	endDone := make(chan struct{})

	go func() {
		endGroup.Wait()
		close(endDone)
	}()

	go func() {
		startGroup.Wait()
		close(startDone)
	}()

	for i := 0; i < 3; i++ {
		go func() {
			cmd := exec.Command(dockerBinary, "attach", "attacher")

			defer func() {
				cmd.Wait()
				endGroup.Done()
			}()

			out, err := cmd.StdoutPipe()
			if err != nil {
				c.Fatal(err)
			}

			if err := cmd.Start(); err != nil {
				c.Fatal(err)
			}

			buf := make([]byte, 1024)

			if _, err := out.Read(buf); err != nil && err != io.EOF {
				c.Fatal(err)
			}

			startGroup.Done()

			if !strings.Contains(string(buf), "hello") {
				c.Fatalf("unexpected output %s expected hello\n", string(buf))
			}
		}()
	}

	select {
	case <-startDone:
	case <-time.After(attachWait):
		c.Fatalf("Attaches did not initialize properly")
	}

	dockerCmd(c, "kill", "attacher")

	select {
	case <-endDone:
	case <-time.After(attachWait):
		c.Fatalf("Attaches did not finish properly")
	}
}

func (s *DockerSuite) TestAttachTTYWithoutStdin(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "-ti", "busybox")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	done := make(chan error)
	go func() {
		defer close(done)

		cmd := exec.Command(dockerBinary, "attach", id)
		if _, err := cmd.StdinPipe(); err != nil {
			done <- err
			return
		}

		expected := "the input device is not a TTY"
		if runtime.GOOS == "windows" {
			expected += ".  If you are using mintty, try prefixing the command with 'winpty'"
		}
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			done <- fmt.Errorf("attach should have failed")
			return
		} else if !strings.Contains(out, expected) {
			done <- fmt.Errorf("attach failed with error %q: expected %q", out, expected)
			return
		}
	}()

	select {
	case err := <-done:
		c.Assert(err, check.IsNil)
	case <-time.After(attachWait):
		c.Fatal("attach is running but should have failed")
	}
}

func (s *DockerSuite) TestAttachDisconnect(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-di", "busybox", "/bin/cat")
	id := strings.TrimSpace(out)

	cmd := exec.Command(dockerBinary, "attach", id)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, check.IsNil)
	defer stdout.Close()
	c.Assert(cmd.Start(), check.IsNil)
	defer cmd.Process.Kill()

	_, err = stdin.Write([]byte("hello\n"))
	c.Assert(err, check.IsNil)
	out, err = bufio.NewReader(stdout).ReadString('\n')
	c.Assert(err, check.IsNil)
	c.Assert(strings.TrimSpace(out), check.Equals, "hello")

	c.Assert(stdin.Close(), check.IsNil)

	// Expect container to still be running after stdin is closed
	running := inspectField(c, id, "State.Running")
	c.Assert(running, check.Equals, "true")
}

func (s *DockerSuite) TestAttachPausedContainer(c *check.C) {
	testRequires(c, DaemonIsLinux) // Containers cannot be paused on Windows
	defer unpauseAllContainers()
	dockerCmd(c, "run", "-d", "--name=test", "busybox", "top")
	dockerCmd(c, "pause", "test")
	out, _, err := dockerCmdWithError("attach", "test")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "You cannot attach to a paused container, unpause it first")
}
