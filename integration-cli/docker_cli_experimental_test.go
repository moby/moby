package main

import (
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestExperimentalVersionTrue(c *check.C) {
	testRequires(c, ExperimentalDaemon)

	out, _ := dockerCmd(c, "version")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Experimental:") {
			c.Assert(line, checker.Matches, "*true")
			return
		}
	}

	c.Fatal(`"Experimental" not found in version output`)
}

func (s *DockerSuite) TestExperimentalVersionFalse(c *check.C) {
	testRequires(c, NotExperimentalDaemon)

	out, _ := dockerCmd(c, "version")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Experimental:") {
			c.Assert(line, checker.Matches, "*false")
			return
		}
	}

	c.Fatal(`"Experimental" not found in version output`)
}
