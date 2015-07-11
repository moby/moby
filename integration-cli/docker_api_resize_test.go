package main

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestResizeApiResponse(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeApiHeightWidthNoInt(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=foo&w=bar"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeApiResponseWhenContainerNotStarted(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	// make sure the exited container is not running
	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf(out, err)
	}

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	status, body, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)

	if !strings.Contains(string(body), "Cannot resize container") && !strings.Contains(string(body), cleanedContainerID) {
		c.Fatalf("resize should fail with message 'Cannot resize container' but instead received %s", string(body))
	}
}
