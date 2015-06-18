// +build experimental

package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func isSrvAvailable(c *check.C, sname string, name string) bool {
	runCmd := exec.Command(dockerBinary, "service", "ls")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ {
		if strings.Contains(lines[i], sname) && strings.Contains(lines[i], name) {
			return true
		}
	}
	return false
}
func isNwAvailable(c *check.C, name string) bool {
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

func (s *DockerSuite) TestDockerServiceCreateDelete(c *check.C) {
	runCmd := exec.Command(dockerBinary, "network", "create", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	if !isNwAvailable(c, "test") {
		c.Fatalf("Network test not found")
	}

	runCmd = exec.Command(dockerBinary, "service", "publish", "s1.test")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	if !isSrvAvailable(c, "s1", "test") {
		c.Fatalf("service s1.test not found")
	}

	runCmd = exec.Command(dockerBinary, "service", "unpublish", "s1.test")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	if isSrvAvailable(c, "s1", "test") {
		c.Fatalf("service s1.test not removed")
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
