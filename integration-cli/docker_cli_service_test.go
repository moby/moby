// +build experimental

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func assertSrvIsAvailable(c *check.C, sname, name string) {
	if !isSrvPresent(c, sname, name) {
		c.Fatalf("Service %s on network %s not found in service ls o/p", sname, name)
	}
}

func assertSrvNotAvailable(c *check.C, sname, name string) {
	if isSrvPresent(c, sname, name) {
		c.Fatalf("Found service %s on network %s in service ls o/p", sname, name)
	}
}

func isSrvPresent(c *check.C, sname, name string) bool {
	runCmd := exec.Command(dockerBinary, "service", "ls")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ {
		if strings.Contains(lines[i], sname) && strings.Contains(lines[i], name) {
			return true
		}
	}
	return false
}

func isCntPresent(c *check.C, cname, sname, name string) bool {
	runCmd := exec.Command(dockerBinary, "service", "ls", "--no-trunc")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ {
		fmt.Println(lines)
		if strings.Contains(lines[i], name) && strings.Contains(lines[i], sname) && strings.Contains(lines[i], cname) {
			return true
		}
	}
	return false
}

func (s *DockerSuite) TestDockerServiceCreateDelete(c *check.C) {
	runCmd := exec.Command(dockerBinary, "network", "create", "test")
	_, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertNwIsAvailable(c, "test")

	runCmd = exec.Command(dockerBinary, "service", "publish", "s1.test")
	_, _, _, err = runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertSrvIsAvailable(c, "s1", "test")

	runCmd = exec.Command(dockerBinary, "service", "unpublish", "s1.test")
	_, _, _, err = runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertSrvNotAvailable(c, "s1", "test")

	runCmd = exec.Command(dockerBinary, "network", "rm", "test")
	_, _, _, err = runCommandWithStdoutStderr(runCmd)
	c.Assert(err, check.IsNil)
	assertNwNotAvailable(c, "test")
}

func (s *DockerSuite) TestDockerPublishServiceFlag(c *check.C) {
	// Run saying the container is the backend for the specified service on the specified network
	runCmd := exec.Command(dockerBinary, "run", "-d", "--expose=23", "--publish-service", "telnet.production", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
	cid := strings.TrimSpace(out)

	// Verify container is attached in service ps o/p
	assertSrvIsAvailable(c, "telnet", "production")
	runCmd = exec.Command(dockerBinary, "rm", "-f", cid)
	out, _, err = runCommandWithOutput(runCmd)
	c.Assert(err, check.IsNil)
}
