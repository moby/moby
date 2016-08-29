// +build !windows

package main

import (
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInfoSecurityOptions(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, Apparmor, DaemonIsLinux)

	out, _ := dockerCmd(c, "info")
	c.Assert(out, checker.Contains, "Security Options: apparmor seccomp")
}
