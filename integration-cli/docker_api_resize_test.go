package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestResizeApiResponse(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()
	cleanedContainerID := stripTrailingCharacters(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	_, err = sockRequest("POST", endpoint, nil)
	if err != nil {
		t.Fatalf("resize Request failed %v", err)
	}

	logDone("container resize - when started")
}

func TestResizeApiResponseWhenContainerNotStarted(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()
	cleanedContainerID := stripTrailingCharacters(out)

	// make sure the exited cintainer is not running
	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	body, err := sockRequest("POST", endpoint, nil)
	if err == nil {
		t.Fatalf("resize should fail when container is not started")
	}
	if !strings.Contains(string(body), "Cannot resize container") && !strings.Contains(string(body), cleanedContainerID) {
		t.Fatalf("resize should fail with message 'Cannot resize container' but instead received %s", string(body))
	}

	logDone("container resize - when not started should not resize")
}
