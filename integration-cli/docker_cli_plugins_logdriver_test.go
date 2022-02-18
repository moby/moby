package main

import (
	"context"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

func (s *DockerSuite) TestPluginLogDriver(c *testing.T) {
	testRequires(c, IsAmd64, DaemonIsLinux)

	pluginName := "cpuguy83/docker-logdriver-test:latest"

	dockerCmd(c, "plugin", "install", pluginName)
	dockerCmd(c, "run", "--log-driver", pluginName, "--name=test", "busybox", "echo", "hello")
	out, _ := dockerCmd(c, "logs", "test")
	assert.Equal(c, strings.TrimSpace(out), "hello")

	dockerCmd(c, "start", "-a", "test")
	out, _ = dockerCmd(c, "logs", "test")
	assert.Equal(c, strings.TrimSpace(out), "hello\nhello")

	dockerCmd(c, "rm", "test")
	dockerCmd(c, "plugin", "disable", pluginName)
	dockerCmd(c, "plugin", "rm", pluginName)
}

// Make sure log drivers are listed in info, and v2 plugins are not.
func (s *DockerSuite) TestPluginLogDriverInfoList(c *testing.T) {
	testRequires(c, IsAmd64, DaemonIsLinux)
	pluginName := "cpuguy83/docker-logdriver-test"

	dockerCmd(c, "plugin", "install", pluginName)

	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer cli.Close()

	info, err := cli.Info(context.Background())
	assert.NilError(c, err)

	drivers := strings.Join(info.Plugins.Log, " ")
	assert.Assert(c, strings.Contains(drivers, "json-file"))
	assert.Assert(c, !strings.Contains(drivers, pluginName))
}
