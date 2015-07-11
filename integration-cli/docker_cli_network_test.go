// +build experimental

package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func assertNwIsAvailable(c *check.C, name string) {
	if !isNwPresent(c, name) {
		c.Fatalf("Network %s not found in network ls o/p", name)
	}
}

func assertNwNotAvailable(c *check.C, name string) {
	if isNwPresent(c, name) {
		c.Fatalf("Found network %s in network ls o/p", name)
	}
}

func isNwPresent(c *check.C, name string) bool {
	runCmd := exec.Command(dockerBinary, "network", "ls")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ {
		if strings.Contains(lines[i], name) {
			return true
		}
	}
	return false
}

func (s *DockerSuite) TestDockerNetworkLsDefault(c *check.C) {
	defaults := []string{"bridge", "host", "none"}
	for _, nn := range defaults {
		assertNwIsAvailable(c, nn)
	}
}

func (s *DockerSuite) TestDockerNetworkCreateDelete(c *check.C) {
	runCmd := exec.Command(dockerBinary, "network", "create", "test")
	_, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertNwIsAvailable(c, "test")

	runCmd = exec.Command(dockerBinary, "network", "rm", "test")
	_, _, _, err = runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertNwNotAvailable(c, "test")
}
