package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestStopContainerWithRestartAlways(c *check.C) {
	dockerCmd(c, "run", "--name", "verifyRestart1", "-d", "--restart=always", "busybox", "false")
	dockerCmd(c, "run", "--name", "verifyRestart2", "-d", "--restart=always", "busybox", "false")
	dockerCmd(c, "run", "--name", "verifyRestart3", "-d", "--restart=always", "busybox", "false")

	c.Assert(waitRun("verifyRestart1"), checker.IsNil)
	c.Assert(waitRun("verifyRestart2"), checker.IsNil)
	c.Assert(waitRun("verifyRestart3"), checker.IsNil)

	dockerCmd(c, "stop", "verifyRestart1")
	dockerCmd(c, "stop", "verifyRestart2")
	dockerCmd(c, "stop", "verifyRestart3")
}
