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

	"github.com/go-check/check"
	"github.com/kr/pty"
)

// #5979
func (s *DockerSuite) TestEventsRedirectStdout(c *check.C) {
	since := daemonTime(c).Unix()
	dockerCmd(c, "run", "busybox", "true")

	file, err := ioutil.TempFile("", "")
	if err != nil {
		c.Fatalf("could not create temp file: %v", err)
	}
	defer os.Remove(file.Name())

	command := fmt.Sprintf("%s events --since=%d --until=%d > %s", dockerBinary, since, daemonTime(c).Unix(), file.Name())
	_, tty, err := pty.Open()
	if err != nil {
		c.Fatalf("Could not open pty: %v", err)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Run(); err != nil {
		c.Fatalf("run err for command %q: %v", command, err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		for _, ch := range scanner.Text() {
			if unicode.IsControl(ch) {
				c.Fatalf("found control character %v", []byte(string(ch)))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		c.Fatalf("Scan err for command %q: %v", command, err)
	}

}

func (s *DockerSuite) TestEventsOOMDisableFalse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, oomControl)

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
		c.Assert(err, check.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("Timeout waiting for container to die on OOM")
	}

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=oomFalse", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(events) < 5 {
		c.Fatalf("Missing expected event")
	}

	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	oomEvent := strings.Fields(events[len(events)-2])
	dieEvent := strings.Fields(events[len(events)-1])
	if createEvent[len(createEvent)-1] != "create" {
		c.Fatalf("event should be create, not %#v", createEvent)
	}
	if attachEvent[len(attachEvent)-1] != "attach" {
		c.Fatalf("event should be attach, not %#v", attachEvent)
	}
	if startEvent[len(startEvent)-1] != "start" {
		c.Fatalf("event should be start, not %#v", startEvent)
	}
	if oomEvent[len(oomEvent)-1] != "oom" {
		c.Fatalf("event should be oom, not %#v", oomEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		c.Fatalf("event should be die, not %#v", dieEvent)
	}
}

func (s *DockerSuite) TestEventsOOMDisableTrue(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, oomControl)

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
		c.Assert(err, check.IsNil)
	case <-time.After(20 * time.Second):
		defer dockerCmd(c, "kill", "oomTrue")

		out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=oomTrue", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
		events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(events) < 4 {
			c.Fatalf("Missing expected event")
		}

		createEvent := strings.Fields(events[len(events)-4])
		attachEvent := strings.Fields(events[len(events)-3])
		startEvent := strings.Fields(events[len(events)-2])
		oomEvent := strings.Fields(events[len(events)-1])

		if createEvent[len(createEvent)-1] != "create" {
			c.Fatalf("event should be create, not %#v", createEvent)
		}
		if attachEvent[len(attachEvent)-1] != "attach" {
			c.Fatalf("event should be attach, not %#v", attachEvent)
		}
		if startEvent[len(startEvent)-1] != "start" {
			c.Fatalf("event should be start, not %#v", startEvent)
		}
		if oomEvent[len(oomEvent)-1] != "oom" {
			c.Fatalf("event should be oom, not %#v", oomEvent)
		}

		out, _ = dockerCmd(c, "inspect", "-f", "{{.State.Status}}", "oomTrue")
		if strings.TrimSpace(out) != "running" {
			c.Fatalf("container should be still running, not %v", out)
		}
	}
}
