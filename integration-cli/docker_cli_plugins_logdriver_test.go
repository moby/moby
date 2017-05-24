package main

import (
	"strings"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"golang.org/x/net/context"
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

// Make sure log drivers are listed in info, and v2 plugins are not.
func (s *DockerSuite) TestPluginLogDriverInfoList(c *check.C) {
	testRequires(c, IsAmd64, DaemonIsLinux)
	pluginName := "cpuguy83/docker-logdriver-test"

	dockerCmd(c, "plugin", "install", pluginName)

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	info, err := cli.Info(context.Background())
	c.Assert(err, checker.IsNil)

	drivers := strings.Join(info.Plugins.Log, " ")
	c.Assert(drivers, checker.Contains, "json-file")
	c.Assert(drivers, checker.Not(checker.Contains), pluginName)
}
