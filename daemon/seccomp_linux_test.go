package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	coci "github.com/containerd/containerd/oci"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	dconfig "github.com/docker/docker/daemon/config"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/profiles/seccomp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"gotest.tools/v3/assert"
)

func TestWithSeccomp(t *testing.T) {
	type expected struct {
		daemon  *Daemon
		c       *container.Container
		inSpec  coci.Spec
		outSpec coci.Spec
		err     string
		comment string
	}

	for _, tc := range []expected{
		{
			comment: "unconfined seccompProfile runs unconfined",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: true},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: dconfig.SeccompProfileUnconfined},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec:  oci.DefaultLinuxSpec(),
			outSpec: oci.DefaultLinuxSpec(),
		},
		{
			// Prior to Docker v27, it had resulted in unconfined.
			// https://github.com/moby/moby/pull/47500
			comment: "privileged container w/ custom profile",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: true},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: `{"defaultAction": "SCMP_ACT_LOG"}`},
				HostConfig: &containertypes.HostConfig{
					Privileged: true,
				},
			},
			inSpec: oci.DefaultLinuxSpec(),
			outSpec: func() coci.Spec {
				s := oci.DefaultLinuxSpec()
				profile := &specs.LinuxSeccomp{
					DefaultAction: specs.LinuxSeccompAction("SCMP_ACT_LOG"),
				}
				s.Linux.Seccomp = profile
				return s
			}(),
		},
		{
			comment: "privileged container w/ default runs unconfined",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: true},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: ""},
				HostConfig: &containertypes.HostConfig{
					Privileged: true,
				},
			},
			inSpec:  oci.DefaultLinuxSpec(),
			outSpec: oci.DefaultLinuxSpec(),
		},
		{
			comment: "privileged container w/ daemon profile runs unconfined",
			daemon: &Daemon{
				sysInfo:        &sysinfo.SysInfo{Seccomp: true},
				seccompProfile: []byte(`{"defaultAction": "SCMP_ACT_ERRNO"}`),
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: ""},
				HostConfig: &containertypes.HostConfig{
					Privileged: true,
				},
			},
			inSpec:  oci.DefaultLinuxSpec(),
			outSpec: oci.DefaultLinuxSpec(),
		},
		{
			comment: "custom profile when seccomp is disabled returns error",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: false},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: `{"defaultAction": "SCMP_ACT_ERRNO"}`},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec:  oci.DefaultLinuxSpec(),
			outSpec: oci.DefaultLinuxSpec(),
			err:     "seccomp is not enabled in your kernel, cannot run a custom seccomp profile",
		},
		{
			comment: "empty profile name loads default profile",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: true},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: ""},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec: oci.DefaultLinuxSpec(),
			outSpec: func() coci.Spec {
				s := oci.DefaultLinuxSpec()
				profile, _ := seccomp.GetDefaultProfile(&s)
				s.Linux.Seccomp = profile
				return s
			}(),
		},
		{
			comment: "load container's profile",
			daemon: &Daemon{
				sysInfo: &sysinfo.SysInfo{Seccomp: true},
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: `{"defaultAction": "SCMP_ACT_ERRNO"}`},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec: oci.DefaultLinuxSpec(),
			outSpec: func() coci.Spec {
				s := oci.DefaultLinuxSpec()
				profile := &specs.LinuxSeccomp{
					DefaultAction: specs.LinuxSeccompAction("SCMP_ACT_ERRNO"),
				}
				s.Linux.Seccomp = profile
				return s
			}(),
		},
		{
			comment: "load daemon's profile",
			daemon: &Daemon{
				sysInfo:        &sysinfo.SysInfo{Seccomp: true},
				seccompProfile: []byte(`{"defaultAction": "SCMP_ACT_ERRNO"}`),
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: ""},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec: oci.DefaultLinuxSpec(),
			outSpec: func() coci.Spec {
				s := oci.DefaultLinuxSpec()
				profile := &specs.LinuxSeccomp{
					DefaultAction: specs.LinuxSeccompAction("SCMP_ACT_ERRNO"),
				}
				s.Linux.Seccomp = profile
				return s
			}(),
		},
		{
			comment: "load prioritise container profile over daemon's",
			daemon: &Daemon{
				sysInfo:        &sysinfo.SysInfo{Seccomp: true},
				seccompProfile: []byte(`{"defaultAction": "SCMP_ACT_ERRNO"}`),
			},
			c: &container.Container{
				SecurityOptions: container.SecurityOptions{SeccompProfile: `{"defaultAction": "SCMP_ACT_LOG"}`},
				HostConfig: &containertypes.HostConfig{
					Privileged: false,
				},
			},
			inSpec: oci.DefaultLinuxSpec(),
			outSpec: func() coci.Spec {
				s := oci.DefaultLinuxSpec()
				profile := &specs.LinuxSeccomp{
					DefaultAction: specs.LinuxSeccompAction("SCMP_ACT_LOG"),
				}
				s.Linux.Seccomp = profile
				return s
			}(),
		},
	} {
		t.Run(tc.comment, func(t *testing.T) {
			opts := WithSeccomp(tc.daemon, tc.c)
			err := opts(nil, nil, nil, &tc.inSpec)

			assert.DeepEqual(t, tc.inSpec, tc.outSpec)
			if tc.err != "" {
				assert.Error(t, err, tc.err)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
