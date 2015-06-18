package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

// ensure docker version works
func (s *DockerSuite) TestVersionEnsureSucceeds(c *check.C) {
	versionCmd := exec.Command(dockerBinary, "version")
	out, _, err := runCommandWithOutput(versionCmd)
	if err != nil {
		c.Fatalf("failed to execute docker version: %s, %v", out, err)
	}

	stringsToCheck := []string{
		"Client version:",
		"Client API version:",
		"Go version (client):",
		"Git commit (client):",
		"OS/Arch (client):",
		"Server version:",
		"Server API version:",
		"Go version (server):",
		"Git commit (server):",
		"OS/Arch (server):",
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			c.Errorf("couldn't find string %v in output", linePrefix)
		}
	}

}
