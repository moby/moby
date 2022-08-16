//go:build linux
// +build linux

package daemon

import (
	"testing"

	"github.com/containerd/containerd/pkg/apparmor"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/exec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/v3/assert"
)

func TestExecSetPlatformOptAppArmor(t *testing.T) {
	appArmorEnabled := apparmor.HostSupports()

	tests := []struct {
		doc             string
		privileged      bool
		appArmorProfile string
		expectedProfile string
	}{
		{
			doc:             "default options",
			expectedProfile: defaultAppArmorProfile,
		},
		{
			doc:             "custom profile",
			appArmorProfile: "my-custom-profile",
			expectedProfile: "my-custom-profile",
		},
		{
			doc:             "privileged container",
			privileged:      true,
			expectedProfile: unconfinedAppArmorProfile,
		},
		{
			doc:             "privileged container, custom profile",
			privileged:      true,
			appArmorProfile: "my-custom-profile",
			expectedProfile: "my-custom-profile",
			// FIXME: execSetPlatformOpts prefers custom profiles over "privileged",
			//        which looks like a bug (--privileged on the container should
			//        disable apparmor, seccomp, and selinux); see the code at:
			//        https://github.com/moby/moby/blob/46cdcd206c56172b95ba5c77b827a722dab426c5/daemon/exec_linux.go#L32-L40
			// expectedProfile: unconfinedAppArmorProfile,
		},
	}

	d := &Daemon{configStore: &config.Config{}}

	// Currently, `docker exec --privileged` inherits the Privileged configuration
	// of the container, and does not disable AppArmor.
	// See https://github.com/moby/moby/pull/31773#discussion_r105586900
	//
	// This behavior may change in future, but to verify the current behavior,
	// we run the test both with "exec" and "exec --privileged", which should
	// both give the same result.
	for _, execPrivileged := range []bool{false, true} {
		for _, tc := range tests {
			tc := tc
			doc := tc.doc
			if !appArmorEnabled {
				// no profile should be set if the host does not support AppArmor
				doc += " (apparmor disabled)"
				tc.expectedProfile = ""
			}
			if execPrivileged {
				doc += " (exec privileged)"
			}
			t.Run(doc, func(t *testing.T) {
				c := &container.Container{
					AppArmorProfile: tc.appArmorProfile,
					HostConfig: &containertypes.HostConfig{
						Privileged: tc.privileged,
					},
				}
				ec := &exec.Config{Privileged: execPrivileged}
				p := &specs.Process{}

				err := d.execSetPlatformOpt(c, ec, p)
				assert.NilError(t, err)
				assert.Equal(t, p.ApparmorProfile, tc.expectedProfile)
			})
		}
	}
}
