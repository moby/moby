package main

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestGetVersion(c *check.C) {
	status, body, err := sockRequest("GET", "/version", nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	var v types.Version

	c.Assert(json.Unmarshal(body, &v), checker.IsNil)

	c.Assert(v.Version, checker.Equals, dockerversion.Version, check.Commentf("Version mismatch"))
}
