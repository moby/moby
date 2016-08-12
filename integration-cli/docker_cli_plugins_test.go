package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"

	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	pName        = "tiborvass/no-remove"
	pTag         = "latest"
	pNameWithTag = pName + ":" + pTag
)

func (s *DockerSuite) TestPluginBasicOps(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
	_, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err := dockerCmdWithError("plugin", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pName)
	c.Assert(out, checker.Contains, pTag)
	c.Assert(out, checker.Contains, "true")

	out, _, err = dockerCmdWithError("plugin", "inspect", pNameWithTag)
	c.Assert(err, checker.IsNil)
	tmpFile, err := ioutil.TempFile("", "inspect.json")
	c.Assert(err, checker.IsNil)
	defer tmpFile.Close()

	if _, err := tmpFile.Write([]byte(out)); err != nil {
		c.Fatal(err)
	}
	// FIXME: When `docker plugin inspect` takes a format as input, jq can be replaced.
	id, err := exec.Command("jq", ".Id", "--raw-output", tmpFile.Name()).CombinedOutput()
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(out, checker.Contains, "is active")

	_, _, err = dockerCmdWithError("plugin", "disable", pNameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, pNameWithTag)

	_, err = os.Stat(filepath.Join(dockerBasePath, "plugins", string(id)))
	if !os.IsNotExist(err) {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestPluginInstallDisable(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
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
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
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
