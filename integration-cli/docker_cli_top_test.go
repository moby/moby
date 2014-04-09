package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestTopNonPrivileged(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox", "sleep", "20")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to start the container: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(topCmd)
	errorOut(err, t, fmt.Sprintf("failed to run top: %v %v", out, err))

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	_, err = runCommand(killCmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container: %v", err))

	deleteContainer(cleanedContainerID)

	if !strings.Contains(out, "sleep 20") {
		t.Fatal("top should've listed sleep 20 in the process list")
	}

	logDone("top - sleep process should be listed in non privileged mode")
}

func TestTopPrivileged(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--privileged", "-i", "-d", "busybox", "sleep", "20")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to start the container: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out, _, err = runCommandWithOutput(topCmd)
	errorOut(err, t, fmt.Sprintf("failed to run top: %v %v", out, err))

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	_, err = runCommand(killCmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container: %v", err))

	deleteContainer(cleanedContainerID)

	if !strings.Contains(out, "sleep 20") {
		t.Fatal("top should've listed sleep 20 in the process list")
	}

	logDone("top - sleep process should be listed in privileged mode")
}
