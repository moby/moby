//go:build !windows

package main

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerCLIInfoSuite) TestInfoSecurityOptions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	if !seccompEnabled() && !Apparmor() {
		c.Skip("test requires Seccomp and/or AppArmor")
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	apiClient, err := client.NewClientWithOpts(ctx, client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close(ctx)
	info, err := apiClient.Info(testutil.GetContext(c))
	assert.NilError(c, err)

	if Apparmor() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=apparmor"))
	}
	if seccompEnabled() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=seccomp,profile="+config.SeccompProfileDefault))
	}
}
