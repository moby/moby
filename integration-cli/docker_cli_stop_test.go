package main

import (
	"os/exec"
	"testing"
)

func TestStopContainerWithRmFlag(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sleep", "100")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	stopCmdWithRm := exec.Command(dockerBinary, "stop", "--rm", cleanedContainerID)
	out, _, err = runCommandWithOutput(stopCmdWithRm)
	if err != nil {
		t.Fatal(out, err)
	}

	outputID := stripTrailingCharacters(out)
	if outputID != cleanedContainerID {
		t.Fatalf("Expected to get %s, got %s", cleanedContainerID, outputID)
	}

	out, err = getAllContainers()
	if err != nil {
		t.Fatal(out, err)
	}

	if out != "" {
		t.Fatal("Expected not to have containers", out)
	}

	logDone("stop - container is removed if stopped with --rm")
}
