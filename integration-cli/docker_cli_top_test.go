package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestTopMultipleArgs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-i", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "top", cleanedContainerID, "-o", "pid")
	c.Assert(out, checker.Contains, "PID", check.Commentf("did not see PID after top -o pid: %s", out))
}

func (s *DockerSuite) TestTopNonPrivileged(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-i", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	out1, _ := dockerCmd(c, "top", cleanedContainerID)
	out2, _ := dockerCmd(c, "top", cleanedContainerID)
	dockerCmd(c, "kill", cleanedContainerID)

	c.Assert(out1, checker.Contains, "top", check.Commentf("top should've listed `top` in the process list, but failed the first time"))
	c.Assert(out2, checker.Contains, "top", check.Commentf("top should've listed `top` in the process list, but failed the second time"))
}

func (s *DockerSuite) TestTopPrivileged(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--privileged", "-i", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	out1, _ := dockerCmd(c, "top", cleanedContainerID)
	out2, _ := dockerCmd(c, "top", cleanedContainerID)
	dockerCmd(c, "kill", cleanedContainerID)

	c.Assert(out1, checker.Contains, "top", check.Commentf("top should've listed `top` in the process list, but failed the first time"))
	c.Assert(out2, checker.Contains, "top", check.Commentf("top should've listed `top` in the process list, but failed the second time"))
}
