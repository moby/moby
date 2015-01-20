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

	cleanedContainerID := stripTrailingCharacters(out)

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

	cleanedContainerID := stripTrailingCharacters(out)

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
