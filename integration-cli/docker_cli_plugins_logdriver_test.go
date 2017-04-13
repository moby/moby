package main

import (
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPluginLogDriver(c *check.C) {
	testRequires(c, IsAmd64, DaemonIsLinux)

	pluginName := "cpuguy83/docker-logdriver-test:latest"

	dockerCmd(c, "plugin", "install", pluginName)
	dockerCmd(c, "run", "--log-driver", pluginName, "--name=test", "busybox", "echo", "hello")
	out, _ := dockerCmd(c, "logs", "test")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello")

	dockerCmd(c, "start", "-a", "test")
	out, _ = dockerCmd(c, "logs", "test")
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello\nhello")

	dockerCmd(c, "rm", "test")
	dockerCmd(c, "plugin", "disable", pluginName)
	dockerCmd(c, "plugin", "rm", pluginName)
}
