package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"

	"os"
	"path/filepath"
	"strings"
)

var (
	pName        = "tiborvass/no-remove"
	pTag         = "latest"
	pNameWithTag = pName + ":" + pTag
)

func (s *DockerSuite) TestPluginBasicOps(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)
	_, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err := dockerCmdWithError("plugin", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pName)
	c.Assert(out, checker.Contains, pTag)
	c.Assert(out, checker.Contains, "true")

	id, _, err := dockerCmdWithError("plugin", "inspect", "-f", "{{.Id}}", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(out, checker.Contains, "is enabled")

	_, _, err = dockerCmdWithError("plugin", "disable", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pNameWithTag)

	_, err = os.Stat(filepath.Join(dockerBasePath, "plugins", id))
	if !os.IsNotExist(err) {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestPluginForceRemove(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)
	out, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(out, checker.Contains, "is enabled")

	out, _, err = dockerCmdWithError("plugin", "remove", "--force", pNameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pNameWithTag)
}

func (s *DockerSuite) TestPluginActive(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)
	out, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("volume", "create", "-d", pNameWithTag)
	c.Assert(err, checker.IsNil)

	vID := strings.TrimSpace(out)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(out, checker.Contains, "is in use")

	_, _, err = dockerCmdWithError("volume", "rm", vID)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(out, checker.Contains, "is enabled")

	_, _, err = dockerCmdWithError("plugin", "disable", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pNameWithTag)
}

func (s *DockerSuite) TestPluginInstallDisable(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)
	out, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", "--disable", pName)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)

	out, _, err = dockerCmdWithError("plugin", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "false")

	out, _, err = dockerCmdWithError("plugin", "enable", pName)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)

	out, _, err = dockerCmdWithError("plugin", "disable", pName)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)

	out, _, err = dockerCmdWithError("plugin", "remove", pName)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)
}

func (s *DockerSuite) TestPluginInstallImage(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
	out, _, err := dockerCmdWithError("plugin", "install", "redis")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "content is not a plugin")
}

func (s *DockerSuite) TestPluginEnableDisableNegative(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon, Network)
	out, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pName)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, pName)

	out, _, err = dockerCmdWithError("plugin", "enable", pName)
	c.Assert(err, checker.NotNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, "already enabled")

	_, _, err = dockerCmdWithError("plugin", "disable", pName)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "disable", pName)
	c.Assert(err, checker.NotNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, "already disabled")

	_, _, err = dockerCmdWithError("plugin", "remove", pName)
	c.Assert(err, checker.IsNil)
}
