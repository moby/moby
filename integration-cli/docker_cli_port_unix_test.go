// +build !windows

package main

import (
	"net"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestPortHostBinding(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-p", "9876:80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if !assertPortList(c, out, []string{"0.0.0.0:9876"}) {
		c.Error("Port list is not correct")
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", "9876")
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Error("Port is still bound after the Container is removed")
	}
}

func (s *DockerSuite) TestPortExposeHostBinding(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "-P", "--expose", "80", "busybox",
		"nc", "-l", "-p", "80")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
	firstID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "port", firstID, "80")
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	_, exposedPort, err := net.SplitHostPort(out)

	if err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "rm", "-f", firstID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		c.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--net=host", "busybox",
		"nc", "localhost", strings.TrimSpace(exposedPort))
	if out, _, err = runCommandWithOutput(runCmd); err == nil {
		c.Error("Port is still bound after the Container is removed")
	}
}
