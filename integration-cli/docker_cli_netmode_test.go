package main

import (
	"strings"
	"testing"

	"github.com/docker/docker/runconfig"
	"gotest.tools/v3/assert"
)

// GH14530. Validates combinations of --net= with other options

// stringCheckPS is how the output of PS starts in order to validate that
// the command executed in a container did really run PS correctly.
const stringCheckPS = "PID   USER"

type DockerCLINetmodeSuite struct {
	ds *DockerSuite
}

func (s *DockerCLINetmodeSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLINetmodeSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// DockerCmdWithFail executes a docker command that is supposed to fail and returns
// the output, the exit code. If the command returns a Nil error, it will fail and
// stop the tests.
func dockerCmdWithFail(c *testing.T, args ...string) (string, int) {
	out, status, err := dockerCmdWithError(args...)
	assert.Assert(c, err != nil, "%v", out)
	return out, status
}

func (s *DockerCLINetmodeSuite) TestNetHostnameWithNetHost(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	out, _ := dockerCmd(c, "run", "--net=host", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, stringCheckPS))
}

func (s *DockerCLINetmodeSuite) TestNetHostname(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-h=name", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, stringCheckPS))
	out, _ = dockerCmd(c, "run", "-h=name", "--net=bridge", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, stringCheckPS))
	out, _ = dockerCmd(c, "run", "-h=name", "--net=none", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, stringCheckPS))
	out, _ = dockerCmdWithFail(c, "run", "-h=name", "--net=container:other", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkHostname.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, "invalid container format container:<name|id>"))
	out, _ = dockerCmdWithFail(c, "run", "--net=weird", "busybox", "ps")
	assert.Assert(c, strings.Contains(strings.ToLower(out), "not found"))
}

func (s *DockerCLINetmodeSuite) TestConflictContainerNetworkAndLinks(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmdWithFail(c, "run", "--net=container:other", "--link=zip:zap", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictContainerNetworkAndLinks.Error()))
}

func (s *DockerCLINetmodeSuite) TestConflictContainerNetworkHostAndLinks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	out, _ := dockerCmdWithFail(c, "run", "--net=host", "--link=zip:zap", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictHostNetworkAndLinks.Error()))
}

func (s *DockerCLINetmodeSuite) TestConflictNetworkModeNetHostAndOptions(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	out, _ := dockerCmdWithFail(c, "run", "--net=host", "--mac-address=92:d0:c6:0a:29:33", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictContainerNetworkAndMac.Error()))
}

func (s *DockerCLINetmodeSuite) TestConflictNetworkModeAndOptions(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmdWithFail(c, "run", "--net=container:other", "--dns=8.8.8.8", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkAndDNS.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container:other", "--add-host=name:8.8.8.8", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkHosts.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container:other", "--mac-address=92:d0:c6:0a:29:33", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictContainerNetworkAndMac.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container:other", "-P", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkPublishPorts.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container:other", "-p", "8080", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkPublishPorts.Error()))
	out, _ = dockerCmdWithFail(c, "run", "--net=container:other", "--expose", "8000-9000", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNetworkExposePorts.Error()))
}
