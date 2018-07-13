// +build linux

package daemon

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/assert"
)

func TestExecSetPlatformOpt(t *testing.T) {
	if !apparmor.IsEnabled() {
		t.Skip("requires AppArmor to be enabled")
	}
	d := &Daemon{}
	c := &container.Container{AppArmorProfile: "my-custom-profile"}
	ec := &exec.Config{}
	p := &specs.Process{}

	err := d.execSetPlatformOpt(c, ec, p)
	assert.NilError(t, err)
	assert.Equal(t, "my-custom-profile", p.ApparmorProfile)
}

// TestExecSetPlatformOptPrivileged verifies that `docker exec --privileged`
// does not disable AppArmor profiles. Exec currently inherits the `Privileged`
// configuration of the container. See https://github.com/moby/moby/pull/31773#discussion_r105586900
//
// This behavior may change in future, but test for the behavior to prevent it
// from being changed accidentally.
func TestExecSetPlatformOptPrivileged(t *testing.T) {
	if !apparmor.IsEnabled() {
		t.Skip("requires AppArmor to be enabled")
	}
	d := &Daemon{}
	c := &container.Container{AppArmorProfile: "my-custom-profile"}
	ec := &exec.Config{Privileged: true}
	p := &specs.Process{}

	err := d.execSetPlatformOpt(c, ec, p)
	assert.NilError(t, err)
	assert.Equal(t, "my-custom-profile", p.ApparmorProfile)

	c.HostConfig = &containertypes.HostConfig{Privileged: true}
	err = d.execSetPlatformOpt(c, ec, p)
	assert.NilError(t, err)
	assert.Equal(t, "unconfined", p.ApparmorProfile)
}
