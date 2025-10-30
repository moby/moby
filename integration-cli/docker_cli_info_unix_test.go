//go:build !windows

package main

import (
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/internal/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerCLIInfoSuite) TestInfoSecurityOptions(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	if !seccompEnabled() && !Apparmor() {
		c.Skip("test requires Seccomp and/or AppArmor")
	}

	apiClient, err := client.New(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()
	result, err := apiClient.Info(testutil.GetContext(c), client.InfoOptions{})
	assert.NilError(c, err)
	info := result.Info

	if Apparmor() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=apparmor"))
	}
	if seccompEnabled() {
		assert.Check(c, is.Contains(info.SecurityOptions, "name=seccomp,profile="+config.SeccompProfileDefault))
	}
}
