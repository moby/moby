package main

import (
	"os/exec"
	"testing"
)

// Regression test for #3364
func TestDockerStartWithPortCollision(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "fail", "-p", "25:25", "busybox", "true")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal(out, stderr, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--name", "conflict", "-dti", "-p", "25:25", "busybox", "sh")
	out, stderr, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal(out, stderr, err)
	}

	startCmd := exec.Command(dockerBinary, "start", "-a", "fail")
	out, stderr, exitCode, err = runCommandWithStdoutStderr(startCmd)
	if err != nil && exitCode != 1 {
		t.Fatal(out, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", "conflict")
	runCommand(killCmd)

	deleteAllContainers()

	logDone("start - -a=true error on port use")
}
