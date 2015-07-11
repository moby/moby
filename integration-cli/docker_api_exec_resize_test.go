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
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)
}
