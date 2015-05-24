package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

// Regression test for https://github.com/docker/docker/issues/7843
func (s *DockerSuite) TestStartAttachReturnsOnError(c *check.C) {

	dockerCmd(c, "run", "-d", "--name", "test", "busybox")
	dockerCmd(c, "wait", "test")

	// Expect this to fail because the above container is stopped, this is what we want
	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "test2", "--link", "test:test", "busybox")); err == nil {
		c.Fatal("Expected error but got none")
	}

	ch := make(chan error)
	go func() {
		// Attempt to start attached to the container that won't start
		// This should return an error immediately since the container can't be started
		if _, err := runCommand(exec.Command(dockerBinary, "start", "-a", "test2")); err == nil {
			ch <- fmt.Errorf("Expected error but got none")
		}
		close(ch)
	}()

	select {
	case err := <-ch:
		c.Assert(err, check.IsNil)
	case <-time.After(time.Second):
		c.Fatalf("Attach did not exit properly")
	}

}

// gh#8555: Exit code should be passed through when using start -a
func (s *DockerSuite) TestStartAttachCorrectExitCode(c *check.C) {

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 2; exit 1")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)

	// make sure the container has exited before trying the "start -a"
	waitCmd := exec.Command(dockerBinary, "wait", out)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("Failed to wait on container: %v", err)
	}

	startCmd := exec.Command(dockerBinary, "start", "-a", out)
	startOut, exitCode, err := runCommandWithOutput(startCmd)
	if err != nil && !strings.Contains("exit status 1", fmt.Sprintf("%s", err)) {
		c.Fatalf("start command failed unexpectedly with error: %v, output: %q", err, startOut)
	}
	if exitCode != 1 {
		c.Fatalf("start -a did not respond with proper exit code: expected 1, got %d", exitCode)
	}

}

func (s *DockerSuite) TestStartAttachSilent(c *check.C) {

	name := "teststartattachcorrectexitcode"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	// make sure the container has exited before trying the "start -a"
	waitCmd := exec.Command(dockerBinary, "wait", name)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("wait command failed with error: %v", err)
	}

	startCmd := exec.Command(dockerBinary, "start", "-a", name)
	startOut, _, err := runCommandWithOutput(startCmd)
	if err != nil {
		c.Fatalf("start command failed unexpectedly with error: %v, output: %q", err, startOut)
	}
	if expected := "test\n"; startOut != expected {
		c.Fatalf("start -a produced unexpected output: expected %q, got %q", expected, startOut)
	}

}

func (s *DockerSuite) TestStartRecordError(c *check.C) {

	// when container runs successfully, we should not have state.Error
	dockerCmd(c, "run", "-d", "-p", "9999:9999", "--name", "test", "busybox", "top")
	stateErr, err := inspectField("test", "State.Error")
	c.Assert(err, check.IsNil)
	if stateErr != "" {
		c.Fatalf("Expected to not have state error but got state.Error(%q)", stateErr)
	}

	// Expect this to fail and records error because of ports conflict
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "test2", "-p", "9999:9999", "busybox", "top"))
	if err == nil {
		c.Fatalf("Expected error but got none, output %q", out)
	}
	stateErr, err = inspectField("test2", "State.Error")
	c.Assert(err, check.IsNil)
	expected := "port is already allocated"
	if stateErr == "" || !strings.Contains(stateErr, expected) {
		c.Fatalf("State.Error(%q) does not include %q", stateErr, expected)
	}

	// Expect the conflict to be resolved when we stop the initial container
	dockerCmd(c, "stop", "test")
	dockerCmd(c, "start", "test2")
	stateErr, err = inspectField("test2", "State.Error")
	c.Assert(err, check.IsNil)
	if stateErr != "" {
		c.Fatalf("Expected to not have state error but got state.Error(%q)", stateErr)
	}

}

func (s *DockerSuite) TestStartPausedContainer(c *check.C) {
	defer unpauseAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "pause", "testing")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "start", "testing")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Cannot start a paused container, try unpause instead.") {
		c.Fatalf("an error should have been shown that you cannot start paused container: %s\n%v", out, err)
	}

}

func (s *DockerSuite) TestStartMultipleContainers(c *check.C) {
	// run a container named 'parent' and create two container link to `parent`
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "parent", "busybox", "top")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(out, err)
	}
	for _, container := range []string{"child_first", "child_second"} {
		cmd = exec.Command(dockerBinary, "create", "--name", container, "--link", "parent:parent", "busybox", "top")
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			c.Fatal(out, err)
		}
	}

	// stop 'parent' container
	cmd = exec.Command(dockerBinary, "stop", "parent")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(out, err)
	}
	out, err := inspectField("parent", "State.Running")
	c.Assert(err, check.IsNil)
	if out != "false" {
		c.Fatal("Container should be stopped")
	}

	// start all the three containers, container `child_first` start first which should be failed
	// container 'parent' start second and then start container 'child_second'
	cmd = exec.Command(dockerBinary, "start", "child_first", "parent", "child_second")
	out, _, err = runCommandWithOutput(cmd)
	if !strings.Contains(out, "Cannot start container child_first") || err == nil {
		c.Fatal("Expected error but got none")
	}

	for container, expected := range map[string]string{"parent": "true", "child_first": "false", "child_second": "true"} {
		out, err := inspectField(container, "State.Running")
		c.Assert(err, check.IsNil)
		if out != expected {
			c.Fatal("Container running state wrong")
		}

	}

}

func (s *DockerSuite) TestStartAttachMultipleContainers(c *check.C) {

	var cmd *exec.Cmd

	// run  multiple containers to test
	for _, container := range []string{"test1", "test2", "test3"} {
		cmd = exec.Command(dockerBinary, "run", "-d", "--name", container, "busybox", "top")
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			c.Fatal(out, err)
		}
	}

	// stop all the containers
	for _, container := range []string{"test1", "test2", "test3"} {
		cmd = exec.Command(dockerBinary, "stop", container)
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			c.Fatal(out, err)
		}
	}

	// test start and attach multiple containers at once, expected error
	for _, option := range []string{"-a", "-i", "-ai"} {
		cmd = exec.Command(dockerBinary, "start", option, "test1", "test2", "test3")
		out, _, err := runCommandWithOutput(cmd)
		if !strings.Contains(out, "You cannot start and attach multiple containers at once.") || err == nil {
			c.Fatal("Expected error but got none")
		}
	}

	// confirm the state of all the containers be stopped
	for container, expected := range map[string]string{"test1": "false", "test2": "false", "test3": "false"} {
		out, err := inspectField(container, "State.Running")
		if err != nil {
			c.Fatal(out, err)
		}
		if out != expected {
			c.Fatal("Container running state wrong")
		}
	}

}
