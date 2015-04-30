package main

import (
	"github.com/go-check/check"
	"os/exec"
	"strings"
)

func (s *DockerSuite) TestSetContainer(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "test-set-container", "-m", "300M", "busybox", "true")
	_, err := runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "set", "-m", "500M", "test-set-container")
	_, err = runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "inspect", "-f", "{{.HostConfig.Memory}}", "test-set-container")
	memory, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	if !strings.EqualFold(strings.Trim(memory, "\n"), "524288000") {
		c.Fatalf("Got the wrong memory value, we got %d, expected 524288000(500M).", memory)
	}
}

func (s *DockerSuite) TestSetContainerInvalidValue(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "test-set-container", "-m", "300M", "busybox", "true")
	_, err := runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "set", "-m", "2M", "test-set-container")
	_, err = runCommand(cmd)
	if err == nil {
		c.Fatal("[set] should failed if we tried to set invalid value.")
	}
}
