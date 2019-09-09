// +build !windows

package main

import (
	"testing"

	"github.com/docker/docker/integration-cli/checker"
	"gotest.tools/assert"
)

func (s *DockerSuite) TestInfoSecurityOptions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, seccompEnabled, Apparmor, DaemonIsLinux)

	out, _ := dockerCmd(c, "info")
	assert.Assert(c, out, checker.Contains, "Security Options:\n apparmor\n seccomp\n  Profile: default\n")
}
