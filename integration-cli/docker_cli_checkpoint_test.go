// +build experimental

package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestCheckpointAndRestore(c *check.C) {
	defer unpauseAllContainers()
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	containerID := strings.TrimSpace(out)
	checkpointCmd := exec.Command(dockerBinary, "checkpoint", containerID)
	out, _, err = runCommandWithOutput(checkpointCmd)
	if err != nil {
		c.Fatalf("failed to checkpoint container: %v, output: %q", err, out)
	}

	out, err = inspectField(containerID, "State.Checkpointed")
	c.Assert(out, check.Equals, "true")

	restoreCmd := exec.Command(dockerBinary, "restore", containerID)
	out, _, _, err = runCommandWithStdoutStderr(restoreCmd)
	if err != nil {
		c.Fatalf("failed to restore container: %v, output: %q", err, out)
	}

	out, err = inspectField(containerID, "State.Checkpointed")
	c.Assert(out, check.Equals, "false")
}
