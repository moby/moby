// +build experimental

package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func isNetworkPresent(c *check.C, name string) bool {
	runCmd := exec.Command(dockerBinary, "network", "ls")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
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
		if !isNetworkPresent(c, nn) {
			c.Fatalf("Missing Default network : %s", nn)
		}
	}
}

func (s *DockerSuite) TestDockerNetworkCreateDelete(c *check.C) {
	runCmd := exec.Command(dockerBinary, "network", "create", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	if !isNetworkPresent(c, "test") {
		c.Fatalf("Network test not found")
	}

	runCmd = exec.Command(dockerBinary, "network", "rm", "test")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	if isNetworkPresent(c, "test") {
		c.Fatalf("Network test is not removed")
	}
}
