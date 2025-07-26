package main

import (
	"context"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLIPluginLogDriverSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIPluginLogDriverSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLIPluginLogDriverSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

func (s *DockerCLIPluginLogDriverSuite) TestPluginLogDriver(c *testing.T) {
	testRequires(c, IsAmd64, DaemonIsLinux)

	const pluginName = "cpuguy83/docker-logdriver-test:latest"

	cli.DockerCmd(c, "plugin", "install", pluginName)
	cli.DockerCmd(c, "run", "--log-driver", pluginName, "--name=test", "busybox", "echo", "hello")
	out := cli.DockerCmd(c, "logs", "test").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello")

	cli.DockerCmd(c, "start", "-a", "test")
	out = cli.DockerCmd(c, "logs", "test").Combined()
	assert.Equal(c, strings.TrimSpace(out), "hello\nhello") //nolint:dupword

	cli.DockerCmd(c, "rm", "test")
	cli.DockerCmd(c, "plugin", "disable", pluginName)
	cli.DockerCmd(c, "plugin", "rm", pluginName)
}

// Make sure log drivers are listed in info, and v2 plugins are not.
func (s *DockerCLIPluginLogDriverSuite) TestPluginLogDriverInfoList(c *testing.T) {
	testRequires(c, IsAmd64, DaemonIsLinux)
	const pluginName = "cpuguy83/docker-logdriver-test"

	cli.DockerCmd(c, "plugin", "install", pluginName)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	info, err := apiClient.Info(testutil.GetContext(c))
	assert.NilError(c, err)

	drivers := strings.Join(info.Plugins.Log, " ")
	assert.Assert(c, is.Contains(drivers, "json-file"))
	assert.Assert(c, !strings.Contains(drivers, pluginName))
}
