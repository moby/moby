package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// non-blocking wait with 0 exit code
func TestWaitNonBlockedExitZero(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	status := "true"
	for i := 0; status != "false"; i++ {
		runCmd = exec.Command(dockerBinary, "inspect", "--format='{{.State.Running}}'", containerID)
		status, _, err = runCommandWithOutput(runCmd)
		if err != nil {
			t.Fatal(status, err)
		}
		status = strings.TrimSpace(status)

		time.Sleep(time.Second)
		if i >= 60 {
			t.Fatal("Container should have stopped by now")
		}
	}

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	logDone("wait - non-blocking wait with 0 exit code")
}

// blocking wait with 0 exit code
func TestWaitBlockedExitZero(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	logDone("wait - blocking wait with 0 exit code")
}

// non-blocking wait with random exit code
func TestWaitNonBlockedExitRandom(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "exit 99")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	status := "true"
	for i := 0; status != "false"; i++ {
		runCmd = exec.Command(dockerBinary, "inspect", "--format='{{.State.Running}}'", containerID)
		status, _, err = runCommandWithOutput(runCmd)
		if err != nil {
			t.Fatal(status, err)
		}
		status = strings.TrimSpace(status)

		time.Sleep(time.Second)
		if i >= 60 {
			t.Fatal("Container should have stopped by now")
		}
	}

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "99" {
		t.Fatal("failed to set up container", out, err)
	}

	logDone("wait - non-blocking wait with random exit code")
}

// blocking wait with random exit code
func TestWaitBlockedExitRandom(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10; exit 99")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	containerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "wait", containerID)
	out, _, err = runCommandWithOutput(runCmd)

	if err != nil || strings.TrimSpace(out) != "99" {
		t.Fatal("failed to set up container", out, err)
	}

	logDone("wait - blocking wait with random exit code")
}
