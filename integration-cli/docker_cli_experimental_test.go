// +build experimental

package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
	"strings"
)

func (s *DockerSuite) TestExperimentalVersion(c *check.C) {
	out, _ := dockerCmd(c, "version")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Experimental (client):") || strings.HasPrefix(line, "Experimental (server):") {
			c.Assert(line, checker.Matches, "*true")
		}
	}

	out, _ = dockerCmd(c, "-v")
	c.Assert(out, checker.Contains, ", experimental", check.Commentf("docker version did not contain experimental"))
}
