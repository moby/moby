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

func (s *DockerSuite) TestUpdateLogOptRunningContainer(c *check.C) {
	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--log-driver=json-file", "--log-opt", "max-file=1", "--log-opt", "max-size=1m", "busybox", "sleep", "300")
	dockerCmd(c, "update", "--log-opt", "max-file=2", "--log-opt", "max-size=2m", name)

	out, _ := dockerCmd(c, "inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", name)
	c.Assert(out, checker.Contains, "max-size:2m")
	c.Assert(out, checker.Contains, "max-file:2")
}

func (s *DockerSuite) TestUpdateLogOptStoppedContainer(c *check.C) {
	name := "test-update-container"

	dockerCmd(c, "run", "--name", name, "--log-driver=json-file", "--log-opt", "max-file=1", "--log-opt", "max-size=1m", "busybox", "ls")
	dockerCmd(c, "update", "--log-opt", "max-file=2", "--log-opt", "max-size=2m", name)

	out, _ := dockerCmd(c, "inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", name)
	c.Assert(out, checker.Contains, "max-size:2m")
	c.Assert(out, checker.Contains, "max-file:2")
}

func (s *DockerSuite) TestUpdateLogOptPausedContainer(c *check.C) {
	name := "test-update-container"

	dockerCmd(c, "run", "-d", "--name", name, "--log-driver=json-file", "--log-opt", "max-file=1", "--log-opt", "max-size=1m", "busybox", "sleep", "300")
	dockerCmd(c, "pause", name)
	dockerCmd(c, "update", "--log-opt", "max-file=2", "--log-opt", "max-size=2m", name)

	out, _ := dockerCmd(c, "inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", name)
	c.Assert(out, checker.Contains, "max-size:2m")
	c.Assert(out, checker.Contains, "max-file:2")

	dockerCmd(c, "unpause", name)

	out, _ = dockerCmd(c, "inspect", "-f", "{{ .HostConfig.LogConfig.Config }}", name)
	c.Assert(out, checker.Contains, "max-size:2m")
	c.Assert(out, checker.Contains, "max-file:2")
}
