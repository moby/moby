package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTopMultipleArgs(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox", "sleep", "20")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID, "-o", "pid")
	out, _, err = runCommandWithOutput(topCmd)
	if err != nil {
		t.Fatalf("failed to run top: %s, %v", out, err)
	}

	if !strings.Contains(out, "PID") {
		t.Fatalf("did not see PID after top -o pid: %s", out)
	}

	logDone("top - multiple arguments")
}

func TestTopNonPrivileged(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-d", "busybox", "sleep", "20")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out1, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		t.Fatalf("failed to run top: %s, %v", out1, err)
	}

	topCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out2, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		t.Fatalf("failed to run top: %s, %v", out2, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}

	deleteContainer(cleanedContainerID)

	if !strings.Contains(out1, "sleep 20") && !strings.Contains(out2, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed twice")
	} else if !strings.Contains(out1, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed the first time")
	} else if !strings.Contains(out2, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed the second itime")
	}

	logDone("top - sleep process should be listed in non privileged mode")
}

func TestTopPrivileged(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--privileged", "-i", "-d", "busybox", "sleep", "20")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to start the container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	topCmd := exec.Command(dockerBinary, "top", cleanedContainerID)
	out1, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		t.Fatalf("failed to run top: %s, %v", out1, err)
	}

	topCmd = exec.Command(dockerBinary, "top", cleanedContainerID)
	out2, _, err := runCommandWithOutput(topCmd)
	if err != nil {
		t.Fatalf("failed to run top: %s, %v", out2, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}

	deleteContainer(cleanedContainerID)

	if !strings.Contains(out1, "sleep 20") && !strings.Contains(out2, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed twice")
	} else if !strings.Contains(out1, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed the first time")
	} else if !strings.Contains(out2, "sleep 20") {
		t.Fatal("top should've listed `sleep 20` in the process list, but failed the second itime")
	}

	logDone("top - sleep process should be listed in privileged mode")
}
