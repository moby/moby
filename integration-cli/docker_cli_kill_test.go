package main

import (
	"strings"
	"time"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestKillDifferentUserContainer(c *check.C) {
	// TODO Windows: Windows does not yet support -u (Feb 2016).
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "-u", "daemon", "-d", "busybox", "top").Combined()
	cleanedContainerID := strings.TrimSpace(out)
	cli.WaitRun(c, cleanedContainerID)

	cli.DockerCmd(c, "kill", cleanedContainerID)
	cli.WaitExited(c, cleanedContainerID, 10*time.Second)

	out = cli.DockerCmd(c, "ps", "-q").Combined()
	c.Assert(out, checker.Not(checker.Contains), cleanedContainerID, check.Commentf("killed container is still running"))

}
