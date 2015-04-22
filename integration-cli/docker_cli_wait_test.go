package main

import (
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

// non-blocking wait with 0 exit code
func (s *DockerSuite) TestWaitNonBlockedExitZero(c *check.C) {

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	status := "true"
	for i := 0; status != "false"; i++ {
		runCmd = exec.Command(dockerBinary, "inspect", "--format='{{.State.Running}}'", containerID)
		status, _, err = runCommandWithOutput(runCmd)
		if err != nil {
			c.Fatal(status, err)
		}
		status = strings.TrimSpace(status)

		time.Sleep(time.Second)
		if i >= 60 {
			c.Fatal("Container should have stopped by now")
		}
	}

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "0" {
		c.Fatal("failed to set up container", out, err)
	}

}

// blocking wait with 0 exit code
func (s *DockerSuite) TestWaitBlockedExitZero(c *check.C) {

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "0" {
		c.Fatal("failed to set up container", out, err)
	}

}

// non-blocking wait with random exit code
func (s *DockerSuite) TestWaitNonBlockedExitRandom(c *check.C) {

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "exit 99")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	status := "true"
	for i := 0; status != "false"; i++ {
		runCmd = exec.Command(dockerBinary, "inspect", "--format='{{.State.Running}}'", containerID)
		status, _, err = runCommandWithOutput(runCmd)
		if err != nil {
			c.Fatal(status, err)
		}
		status = strings.TrimSpace(status)

		time.Sleep(time.Second)
		if i >= 60 {
			c.Fatal("Container should have stopped by now")
		}
	}

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "99" {
		c.Fatal("failed to set up container", out, err)
	}

}

// blocking wait with random exit code
func (s *DockerSuite) TestWaitBlockedExitRandom(c *check.C) {

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10; exit 99")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "99" {
		c.Fatal("failed to set up container", out, err)
	}

}
