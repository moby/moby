package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Regression test for https://github.com/docker/docker/issues/7843
func TestStartAttachReturnsOnError(t *testing.T) {
	defer deleteAllContainers()

	dockerCmd(t, "run", "-d", "--name", "test", "busybox")
	dockerCmd(t, "wait", "test")

	// Expect this to fail because the above container is stopped, this is what we want
	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "test2", "--link", "test:test", "busybox")); err == nil {
		t.Fatal("Expected error but got none")
	}

	ch := make(chan struct{})
	go func() {
		// Attempt to start attached to the container that won't start
		// This should return an error immediately since the container can't be started
		if _, err := runCommand(exec.Command(dockerBinary, "start", "-a", "test2")); err == nil {
			t.Fatal("Expected error but got none")
		}
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("Attach did not exit properly")
	}

	logDone("start - error on start with attach exits")
}

// gh#8555: Exit code should be passed through when using start -a
func TestStartAttachCorrectExitCode(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 2; exit 1")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = stripTrailingCharacters(out)

	// make sure the container has exited before trying the "start -a"
	waitCmd := exec.Command(dockerBinary, "wait", out)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("Failed to wait on container: %v", err)
	}

	startCmd := exec.Command(dockerBinary, "start", "-a", out)
	startOut, exitCode, err := runCommandWithOutput(startCmd)
	if err != nil && !strings.Contains("exit status 1", fmt.Sprintf("%s", err)) {
		t.Fatalf("start command failed unexpectedly with error: %v, output: %q", err, startOut)
	}
	if exitCode != 1 {
		t.Fatalf("start -a did not respond with proper exit code: expected 1, got %d", exitCode)
	}

	logDone("start - correct exit code returned with -a")
}

func TestStartSilentAttach(t *testing.T) {
	defer deleteAllContainers()

	name := "teststartattachcorrectexitcode"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	// make sure the container has exited before trying the "start -a"
	waitCmd := exec.Command(dockerBinary, "wait", name)
	if _, _, err = runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("wait command failed with error: %v", err)
	}

	startCmd := exec.Command(dockerBinary, "start", "-a", name)
	startOut, _, err := runCommandWithOutput(startCmd)
	if err != nil {
		t.Fatalf("start command failed unexpectedly with error: %v, output: %q", err, startOut)
	}
	if expected := "test\n"; startOut != expected {
		t.Fatalf("start -a produced unexpected output: expected %q, got %q", expected, startOut)
	}

	logDone("start - don't echo container ID when attaching")
}

func TestStartRecordError(t *testing.T) {
	defer deleteAllContainers()

	// when container runs successfully, we should not have state.Error
	dockerCmd(t, "run", "-d", "-p", "9999:9999", "--name", "test", "busybox", "top")
	stateErr, err := inspectField("test", "State.Error")
	if err != nil {
		t.Fatalf("Failed to inspect %q state's error, got error %q", "test", err)
	}
	if stateErr != "" {
		t.Fatalf("Expected to not have state error but got state.Error(%q)", stateErr)
	}

	// Expect this to fail and records error because of ports conflict
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "test2", "-p", "9999:9999", "busybox", "top"))
	if err == nil {
		t.Fatalf("Expected error but got none, output %q", out)
	}
	stateErr, err = inspectField("test2", "State.Error")
	if err != nil {
		t.Fatalf("Failed to inspect %q state's error, got error %q", "test2", err)
	}
	expected := "port is already allocated"
	if stateErr == "" || !strings.Contains(stateErr, expected) {
		t.Fatalf("State.Error(%q) does not include %q", stateErr, expected)
	}

	// Expect the conflict to be resolved when we stop the initial container
	dockerCmd(t, "stop", "test")
	dockerCmd(t, "start", "test2")
	stateErr, err = inspectField("test2", "State.Error")
	if err != nil {
		t.Fatalf("Failed to inspect %q state's error, got error %q", "test", err)
	}
	if stateErr != "" {
		t.Fatalf("Expected to not have state error but got state.Error(%q)", stateErr)
	}

	logDone("start - set state error when start is unsuccessful")
}

// gh#8726: a failed Start() breaks --volumes-from on subsequent Start()'s
func TestStartVolumesFromFailsCleanly(t *testing.T) {
	defer deleteAllContainers()

	// Create the first data volume
	dockerCmd(t, "run", "-d", "--name", "data_before", "-v", "/foo", "busybox")

	// Expect this to fail because the data test after contaienr doesn't exist yet
	if _, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "consumer", "--volumes-from", "data_before", "--volumes-from", "data_after", "busybox")); err == nil {
		t.Fatal("Expected error but got none")
	}

	// Create the second data volume
	dockerCmd(t, "run", "-d", "--name", "data_after", "-v", "/bar", "busybox")

	// Now, all the volumes should be there
	dockerCmd(t, "start", "consumer")

	// Check that we have the volumes we want
	out, _, _ := dockerCmd(t, "inspect", "--format='{{ len .Volumes }}'", "consumer")
	n_volumes := strings.Trim(out, " \r\n'")
	if n_volumes != "2" {
		t.Fatalf("Missing volumes: expected 2, got %s", n_volumes)
	}

	logDone("start - missing containers in --volumes-from did not affect subsequent runs")
}

func TestStartPausedContainer(t *testing.T) {
	defer deleteAllContainers()
	defer unpauseAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "testing", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "pause", "testing")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "start", "testing")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Cannot start a paused container, try unpause instead.") {
		t.Fatalf("an error should have been shown that you cannot start paused container: %s\n%v", out, err)
	}

	logDone("start - error should show if trying to start paused container")
}

func TestStartMultipleContainers(t *testing.T) {
	defer deleteAllContainers()
	// run a container named 'parent' and create two container link to `parent`
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "parent", "busybox", "top")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(out, err)
	}
	for _, container := range []string{"child_first", "child_second"} {
		cmd = exec.Command(dockerBinary, "create", "--name", container, "--link", "parent:parent", "busybox", "top")
		if out, _, err := runCommandWithOutput(cmd); err != nil {
			t.Fatal(out, err)
		}
	}

	// stop 'parent' container
	cmd = exec.Command(dockerBinary, "stop", "parent")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(out, err)
	}
	cmd = exec.Command(dockerBinary, "inspect", "-f", "{{.State.Running}}", "parent")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	out = strings.Trim(out, "\r\n")
	if out != "false" {
		t.Fatal("Container should be stopped")
	}

	// start all the three containers, container `child_first` start first which should be faild
	// container 'parent' start second and then start container 'child_second'
	cmd = exec.Command(dockerBinary, "start", "child_first", "parent", "child_second")
	out, _, err = runCommandWithOutput(cmd)
	if !strings.Contains(out, "Cannot start container child_first") || err == nil {
		t.Fatal("Expected error but got none")
	}

	for container, expected := range map[string]string{"parent": "true", "child_first": "false", "child_second": "true"} {
		cmd = exec.Command(dockerBinary, "inspect", "-f", "{{.State.Running}}", container)
		out, _, err = runCommandWithOutput(cmd)
		if err != nil {
			t.Fatal(out, err)
		}
		out = strings.Trim(out, "\r\n")
		if out != expected {
			t.Fatal("Container running state wrong")
		}

	}

	logDone("start - start multiple containers continue on one failed")
}
