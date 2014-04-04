package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// This used to work, it test a log of PageSize-1 (gh#4851)
func TestLogsContainerSmallerThanPage(t *testing.T) {
	testLen := 32767
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo smaller than page size")
}

// Regression test: When going over the PageSize, it used to panic (gh#4851)
func TestLogsContainerBiggerThanPage(t *testing.T) {
	testLen := 32768
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo bigger than page size")
}

// Regression test: When going much over the PageSize, it used to block (gh#4851)
func TestLogsContainerMuchBiggerThanPage(t *testing.T) {
	testLen := 33000
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n =; done; echo", testLen))
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)
	exec.Command(dockerBinary, "wait", cleanedContainerID).Run()

	logsCmd := exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(logsCmd)
	errorOut(err, t, fmt.Sprintf("failed to log container: %v %v", out, err))

	if len(out) != testLen+1 {
		t.Fatalf("Expected log length of %d, received %d\n", testLen+1, len(out))
	}

	deleteContainer(cleanedContainerID)

	logDone("logs - logs container running echo much bigger than page size")
}
