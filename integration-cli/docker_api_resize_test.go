package main

import (
	"net/http"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestResizeApiResponse(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeApiHeightWidthNoInt(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=foo&w=bar"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestResizeApiResponseWhenContainerNotStarted(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	// make sure the exited container is not running
	dockerCmd(c, "wait", cleanedContainerID)

	endpoint := "/containers/" + cleanedContainerID + "/resize?h=40&w=40"
	status, body, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusInternalServerError)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), checker.Contains, "is not running", check.Commentf("resize should fail with message 'Container is not running'"))
}
