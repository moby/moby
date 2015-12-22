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
	nEvents := len(events)

	c.Assert(nEvents, checker.GreaterOrEqualThan, 5) //Missing expected event
	c.Assert(parseEventAction(c, events[nEvents-5]), checker.Equals, "create")
	c.Assert(parseEventAction(c, events[nEvents-4]), checker.Equals, "attach")
	c.Assert(parseEventAction(c, events[nEvents-3]), checker.Equals, "start")
	c.Assert(parseEventAction(c, events[nEvents-2]), checker.Equals, "oom")
	c.Assert(parseEventAction(c, events[nEvents-1]), checker.Equals, "die")
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
		nEvents := len(events)
		c.Assert(nEvents, checker.GreaterOrEqualThan, 4) //Missing expected event

		c.Assert(parseEventAction(c, events[nEvents-4]), checker.Equals, "create")
		c.Assert(parseEventAction(c, events[nEvents-3]), checker.Equals, "attach")
		c.Assert(parseEventAction(c, events[nEvents-2]), checker.Equals, "start")
		c.Assert(parseEventAction(c, events[nEvents-1]), checker.Equals, "oom")

		out, _ = dockerCmd(c, "inspect", "-f", "{{.State.Status}}", "oomTrue")
		c.Assert(strings.TrimSpace(out), checker.Equals, "running", check.Commentf("container should be still running"))
	}
}

// #18453
func (s *DockerSuite) TestEventsContainerFilterByName(c *check.C) {
	testRequires(c, DaemonIsLinux)
	cOut, _ := dockerCmd(c, "run", "--name=foo", "-d", "busybox", "top")
	c1 := strings.TrimSpace(cOut)
	waitRun("foo")
	cOut, _ = dockerCmd(c, "run", "--name=bar", "-d", "busybox", "top")
	c2 := strings.TrimSpace(cOut)
	waitRun("bar")
	out, _ := dockerCmd(c, "events", "-f", "container=foo", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	c.Assert(out, checker.Contains, c1, check.Commentf(out))
	c.Assert(out, checker.Not(checker.Contains), c2, check.Commentf(out))
}

// #18453
func (s *DockerSuite) TestEventsContainerFilterBeforeCreate(c *check.C) {
	testRequires(c, DaemonIsLinux)
	var (
		out string
		ch  chan struct{}
	)
	ch = make(chan struct{})

	// calculate the time it takes to create and start a container and sleep 2 seconds
	// this is to make sure the docker event will recevie the event of container
	since := daemonTime(c).Unix()
	id, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cID := strings.TrimSpace(id)
	waitRun(cID)
	time.Sleep(2 * time.Second)
	duration := daemonTime(c).Unix() - since

	go func() {
		out, _ = dockerCmd(c, "events", "-f", "container=foo", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()+2*duration))
		close(ch)
	}()
	// Sleep 2 second to wait docker event to start
	time.Sleep(2 * time.Second)
	id, _ = dockerCmd(c, "run", "--name=foo", "-d", "busybox", "top")
	cID = strings.TrimSpace(id)
	waitRun(cID)
	<-ch
	c.Assert(out, checker.Contains, cID, check.Commentf("Missing event of container (foo)"))
}

func (s *DockerSuite) TestVolumeEvents(c *check.C) {
	testRequires(c, DaemonIsLinux)

	since := daemonTime(c).Unix()

	// Observe create/mount volume actions
	dockerCmd(c, "volume", "create", "--name", "test-event-volume-local")
	dockerCmd(c, "run", "--name", "test-volume-container", "--volume", "test-event-volume-local:/foo", "-d", "busybox", "true")
	waitRun("test-volume-container")

	// Observe unmount/destroy volume actions
	dockerCmd(c, "rm", "-f", "test-volume-container")
	dockerCmd(c, "volume", "rm", "test-event-volume-local")

	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(events), checker.GreaterThan, 4)

	volumeEvents := eventActionsByIDAndType(c, events, "test-event-volume-local", "volume")
	c.Assert(volumeEvents, checker.HasLen, 4)
	c.Assert(volumeEvents[0], checker.Equals, "create")
	c.Assert(volumeEvents[1], checker.Equals, "mount")
	c.Assert(volumeEvents[2], checker.Equals, "unmount")
	c.Assert(volumeEvents[3], checker.Equals, "destroy")
}
