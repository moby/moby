package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestKillContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}

	listRunningContainersCmd := exec.Command(dockerBinary, "ps", "-q")
	out, _, err = runCommandWithOutput(listRunningContainersCmd)
	if err != nil {
		t.Fatalf("failed to list running containers: %s, %v", out, err)
	}

	if strings.Contains(out, cleanedContainerID) {
		t.Fatal("killed container is still running")
	}

	deleteContainer(cleanedContainerID)

	logDone("kill - kill container running sleep 10")
}

func TestKillDifferentUserContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-u", "daemon", "-d", "busybox", "sh", "-c", "sleep 10")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	if out, _, err = runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	if out, _, err = runCommandWithOutput(killCmd); err != nil {
		t.Fatalf("failed to kill container: %s, %v", out, err)
	}

	listRunningContainersCmd := exec.Command(dockerBinary, "ps", "-q")
	out, _, err = runCommandWithOutput(listRunningContainersCmd)
	if err != nil {
		t.Fatalf("failed to list running containers: %s, %v", out, err)
	}

	if strings.Contains(out, cleanedContainerID) {
		t.Fatal("killed container is still running")
	}

	deleteContainer(cleanedContainerID)

	logDone("kill - kill container running sleep 10 from a different user")
}

func TestKillNotRunningContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)
	defer deleteContainer(cleanedContainerID)

	if err := waitInspect(cleanedContainerID, "{{.State.Running}}", "false", 5); err != nil {
		t.Fatal(err)
	}

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	out, exitCode, err := runCommandWithOutput(killCmd)
	if err == nil {
		t.Fatalf("expected to get an error, got %s", out)
	}

	if exitCode != 1 {
		t.Fatalf("expected to have exit code 1, got %d", exitCode)
	}

	if !strings.Contains(out, "Cannot kill not running container") {
		t.Fatalf("expected output contains cannot kill not running container ID, got %s", out)
	}

	logDone("kill - kill not running container error")
}
