package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestPluginBasicOps(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
	name := "tiborvass/no-remove"
	tag := "latest"
	nameWithTag := name + ":" + tag

	_, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", name)
	c.Assert(err, checker.IsNil)

	out, _, err := dockerCmdWithError("plugin", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)
	c.Assert(out, checker.Contains, tag)
	c.Assert(out, checker.Contains, "true")

	out, _, err = dockerCmdWithError("plugin", "inspect", nameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "A test plugin for Docker")

	out, _, err = dockerCmdWithError("plugin", "remove", nameWithTag)
	c.Assert(out, checker.Contains, "is active")

	_, _, err = dockerCmdWithError("plugin", "disable", nameWithTag)
	c.Assert(err, checker.IsNil)

	out, _, err = dockerCmdWithError("plugin", "remove", nameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, nameWithTag)
}

func (s *DockerSuite) TestPluginInstallDisable(c *check.C) {
	testRequires(c, DaemonIsLinux, ExperimentalDaemon)
	name := "tiborvass/no-remove"
	tag := "latest"
	nameWithTag := name + ":" + tag

	_, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", "--disable", name)
	c.Assert(err, checker.IsNil)

	out, _, err := dockerCmdWithError("plugin", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "false")

	out, _, err = dockerCmdWithError("plugin", "remove", nameWithTag)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, nameWithTag)
}
