package template

import (
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
)

// New returns the docker default configuration for libcontainer
func New() *libcontainer.Container {
	container := &libcontainer.Container{
		Capabilities: []string{
			"CHOWN",
			"DAC_OVERRIDE",
			"FOWNER",
			"MKNOD",
			"NET_RAW",
			"SETGID",
			"SETUID",
			"SETFCAP",
			"SETPCAP",
			"NET_BIND_SERVICE",
			"SYS_CHROOT",
		},
		Namespaces: map[string]bool{
			"NEWNS":  true,
			"NEWUTS": true,
			"NEWIPC": true,
			"NEWPID": true,
			"NEWNET": true,
		},
		Cgroups: &cgroups.Cgroup{
			Parent:          "docker",
			AllowAllDevices: false,
		},
		Context: libcontainer.Context{},
	}
	if apparmor.IsEnabled() {
		container.Context["apparmor_profile"] = "docker-default"
	}
	return container
}
