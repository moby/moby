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
		if strings.HasPrefix(line, "Experimental (client):") || strings.HasPrefix(line, "Experimental (server):") {
			c.Assert(line, check.Matches, "*true")
		}
	}

	versionCmd = exec.Command(dockerBinary, "-v")
	if out, _, err = runCommandWithOutput(versionCmd); err != nil || !strings.Contains(out, ", experimental") {
		c.Fatalf("docker version did not contain experimental: %s, %v", out, err)
	}
}
