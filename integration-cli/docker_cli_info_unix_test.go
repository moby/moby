//go:build !windows
// +build !windows

package main

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/daemon/config"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerSuite) TestInfoSecurityOptions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	if !seccompEnabled() && !Apparmor() {
		c.Skip("test requires Seccomp and/or AppArmor")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer cli.Close()
	info, err := cli.Info(context.Background())
	assert.NilError(c, err)

	if Apparmor() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=apparmor"))
	}
	if seccompEnabled() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=seccomp,profile="+config.SeccompProfileDefault))
	}
}
