package main

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestExecResizeApiHeightWidthNoInt(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/exec/" + cleanedContainerID + "/resize?h=foo&w=bar"
	status, _, err := sockRequest("POST", endpoint, nil)
	if err == nil {
		c.Fatal("Expected exec resize Request to fail")
	}
	if status != http.StatusInternalServerError {
		c.Fatalf("Status expected %d, got %d", http.StatusInternalServerError, status)
	}
}
