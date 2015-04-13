package main

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func TestExecResizeApiHeightWidthNoInt(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	defer deleteAllContainers()
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/exec/" + cleanedContainerID + "/resize?h=foo&w=bar"
	status, _, err := sockRequest("POST", endpoint, nil)
	if err == nil {
		t.Fatal("Expected exec resize Request to fail")
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("Status expected %d, got %d", http.StatusInternalServerError, status)
	}

	logDone("container exec resize - height, width no int fail")
}
