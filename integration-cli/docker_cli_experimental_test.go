// +build experimental

package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestExperimentalVersion(c *check.C) {
	versionCmd := exec.Command(dockerBinary, "version")
	out, _, err := runCommandWithOutput(versionCmd)
	if err != nil {
		c.Fatalf("failed to execute docker version: %s, %v", out, err)
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Client version:") || strings.HasPrefix(line, "Server version:") {
			c.Assert(line, check.Matches, "*-experimental")
		}
	}
}
