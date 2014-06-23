package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestKillContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "sleep 10")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("run failed with errors: %v", err))

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("out should've been a container id: %v %v", inspectOut, err))

	killCmd := exec.Command(dockerBinary, "kill", cleanedContainerID)
	out, _, err = runCommandWithOutput(killCmd)
	errorOut(err, t, fmt.Sprintf("failed to kill container: %v %v", out, err))

	listRunningContainersCmd := exec.Command(dockerBinary, "ps", "-q")
	out, _, err = runCommandWithOutput(listRunningContainersCmd)
	errorOut(err, t, fmt.Sprintf("failed to list running containers: %v", err))

	if strings.Contains(out, cleanedContainerID) {
		t.Fatal("killed container is still running")
	}

	deleteContainer(cleanedContainerID)

	logDone("kill - kill container running sleep 10")
}
