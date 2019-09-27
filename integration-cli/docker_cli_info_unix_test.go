// +build !windows

package main

import (
	"strings"
	"testing"

	"gotest.tools/assert"
)

func (s *DockerSuite) TestInfoSecurityOptions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, seccompEnabled, Apparmor, DaemonIsLinux)

	out, _ := dockerCmd(c, "info")
	assert.Assert(c, strings.Contains(out, "Security Options:\n apparmor\n seccomp\n  Profile: default\n"))
}
