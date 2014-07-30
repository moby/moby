package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDockerRestartStoppedContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "echo", "foobar")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if out != "foobar\n" {
		t.Errorf("container should've printed 'foobar'")
	}

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if out != "foobar\nfoobar\n" {
		t.Errorf("container should've printed 'foobar' twice")
	}

	deleteAllContainers()

	logDone("restart - echo foobar for stopped container")
}

func TestDockerRestartRunningContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "echo foobar && sleep 30 && echo 'should not print this'")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	time.Sleep(1 * time.Second)

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if out != "foobar\n" {
		t.Errorf("container should've printed 'foobar'")
	}

	runCmd = exec.Command(dockerBinary, "restart", "-t", "1", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	time.Sleep(1 * time.Second)

	if out != "foobar\nfoobar\n" {
		t.Errorf("container should've printed 'foobar' twice")
	}

	deleteAllContainers()

	logDone("restart - echo foobar for running container")
}

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func TestDockerRestartWithVolumes(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-v", "/test", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ len .Volumes }}", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if out = strings.Trim(out, " \n\r"); out != "1" {
		t.Errorf("expect 1 volume received %s", out)
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ .Volumes }}", cleanedContainerID)
	volumes, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, volumes)

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ len .Volumes }}", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	errorOut(err, t, out)

	if out = strings.Trim(out, " \n\r"); out != "1" {
		t.Errorf("expect 1 volume after restart received %s", out)
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ .Volumes }}", cleanedContainerID)
	volumesAfterRestart, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, volumesAfterRestart)

	if volumes != volumesAfterRestart {
		volumes = strings.Trim(volumes, " \n\r")
		volumesAfterRestart = strings.Trim(volumesAfterRestart, " \n\r")
		t.Errorf("expected volume path: %s Actual path: %s", volumes, volumesAfterRestart)
	}

	deleteAllContainers()

	logDone("restart - does not create a new volume on restart")
}
