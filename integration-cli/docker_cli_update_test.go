package main

import (
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestUpdateRestartPolicy(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:3", "busybox", "sh", "-c", "sleep 1 && false")
	timeout := 60 * time.Second
	if daemonPlatform == "windows" {
		timeout = 180 * time.Second
	}

	id := strings.TrimSpace(string(out))

	// update restart policy to on-failure:5
	dockerCmd(c, "update", "--restart=on-failure:5", id)

	err := waitExited(id, timeout)
	c.Assert(err, checker.IsNil)

	count := inspectField(c, id, "RestartCount")
	c.Assert(count, checker.Equals, "5")

	maximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(maximumRetryCount, checker.Equals, "5")
}

func (s *DockerSuite) TestUpdateRestartWithAutoRemoveFlag(c *check.C) {
	out, _ := runSleepingContainer(c, "--rm")
	id := strings.TrimSpace(out)

	// update restart policy for an AutoRemove container
	out, _, err := dockerCmdWithError("update", "--restart=always", id)
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "Restart policy cannot be updated because AutoRemove is enabled for the container")
}
