// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// #5979
func (s *DockerSuite) TestEventsRedirectStdout(c *check.C) {
	since := daemonTime(c).Unix()
	dockerCmd(c, "run", "busybox", "true")

	file, err := ioutil.TempFile("", "")
	c.Assert(err, checker.IsNil, check.Commentf("could not create temp file"))
	defer os.Remove(file.Name())

	command := fmt.Sprintf("%s events --since=%d --until=%d > %s", dockerBinary, since, daemonTime(c).Unix(), file.Name())
	_, tty, err := pty.Open()
	c.Assert(err, checker.IsNil, check.Commentf("Could not open pty"))
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	c.Assert(cmd.Run(), checker.IsNil, check.Commentf("run err for command %q", command))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		for _, ch := range scanner.Text() {
			c.Assert(unicode.IsControl(ch), checker.False, check.Commentf("found control character %v", []byte(string(ch))))
		}
	}
	c.Assert(scanner.Err(), checker.IsNil, check.Commentf("Scan err for command %q", command))

}

func (s *DockerSuite) TestEventsOOMDisableFalse(c *check.C) {
	testRequires(c, DaemonIsLinux, oomControl, memoryLimitSupport, NotGCCGO)

	errChan := make(chan error)
	go func() {
		defer close(errChan)
		out, exitCode, _ := dockerCmdWithError("run", "--name", "oomFalse", "-m", "10MB", "busybox", "sh", "-c", "x=a; while true; do x=$x$x$x$x; done")
		if expected := 137; exitCode != expected {
			errChan <- fmt.Errorf("wrong exit code for OOM container: expected %d, got %d (output: %q)", expected, exitCode, out)
		}
	}()
	select {
	case err := <-errChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("Timeout waiting for container to die on OOM")
	}

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=oomFalse", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	c.Assert(len(events), checker.GreaterOrEqualThan, 5) //Missing expected event

	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	oomEvent := strings.Fields(events[len(events)-2])
	dieEvent := strings.Fields(events[len(events)-1])
	c.Assert(createEvent[len(createEvent)-1], checker.Equals, "create", check.Commentf("event should be create, not %#v", createEvent))
	c.Assert(attachEvent[len(attachEvent)-1], checker.Equals, "attach", check.Commentf("event should be attach, not %#v", attachEvent))
	c.Assert(startEvent[len(startEvent)-1], checker.Equals, "start", check.Commentf("event should be start, not %#v", startEvent))
	c.Assert(oomEvent[len(oomEvent)-1], checker.Equals, "oom", check.Commentf("event should be oom, not %#v", oomEvent))
	c.Assert(dieEvent[len(dieEvent)-1], checker.Equals, "die", check.Commentf("event should be die, not %#v", dieEvent))
}

func (s *DockerSuite) TestEventsOOMDisableTrue(c *check.C) {
	testRequires(c, DaemonIsLinux, oomControl, memoryLimitSupport, NotGCCGO)

	errChan := make(chan error)
	go func() {
		defer close(errChan)
		out, exitCode, _ := dockerCmdWithError("run", "--oom-kill-disable=true", "--name", "oomTrue", "-m", "10MB", "busybox", "sh", "-c", "x=a; while true; do x=$x$x$x$x; done")
		if expected := 137; exitCode != expected {
			errChan <- fmt.Errorf("wrong exit code for OOM container: expected %d, got %d (output: %q)", expected, exitCode, out)
		}
	}()
	select {
	case err := <-errChan:
		c.Assert(err, checker.IsNil)
	case <-time.After(20 * time.Second):
		defer dockerCmd(c, "kill", "oomTrue")

		out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=oomTrue", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
		events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		c.Assert(len(events), checker.GreaterOrEqualThan, 4) //Missing expected event

		createEvent := strings.Fields(events[len(events)-4])
		attachEvent := strings.Fields(events[len(events)-3])
		startEvent := strings.Fields(events[len(events)-2])
		oomEvent := strings.Fields(events[len(events)-1])

		c.Assert(createEvent[len(createEvent)-1], checker.Equals, "create", check.Commentf("event should be create, not %#v", createEvent))
		c.Assert(attachEvent[len(attachEvent)-1], checker.Equals, "attach", check.Commentf("event should be attach, not %#v", attachEvent))
		c.Assert(startEvent[len(startEvent)-1], checker.Equals, "start", check.Commentf("event should be start, not %#v", startEvent))
		c.Assert(oomEvent[len(oomEvent)-1], checker.Equals, "oom", check.Commentf("event should be oom, not %#v", oomEvent))

		out, _ = dockerCmd(c, "inspect", "-f", "{{.State.Status}}", "oomTrue")
		c.Assert(strings.TrimSpace(out), checker.Equals, "running", check.Commentf("container should be still running"))
	}
}
