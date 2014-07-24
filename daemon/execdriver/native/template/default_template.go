package template

import (
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/cgroups"
)

// New returns the docker default configuration for libcontainer
func New() *libcontainer.Config {
	container := &libcontainer.Config{
		Capabilities: []string{
			"CHOWN",
			"DAC_OVERRIDE",
			"FSETID",
			"FOWNER",
			"MKNOD",
			"NET_RAW",
			"SETGID",
			"SETUID",
			"SETFCAP",
			"SETPCAP",
			"NET_BIND_SERVICE",
			"SYS_CHROOT",
			"KILL",
			"AUDIT_WRITE",
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
		MountConfig: &libcontainer.MountConfig{},
	}

	if apparmor.IsEnabled() {
		container.AppArmorProfile = "docker-default"
	}

	return container
}
