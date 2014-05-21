package template

import (
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer/mount/nodes"
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
		},
		Namespaces: map[string]bool{
			"NEWNS":  true,
			"NEWUTS": true,
			"NEWIPC": true,
			"NEWPID": true,
			"NEWNET": true,
		},
		Cgroups: &cgroups.Cgroup{
			Parent:       "docker",
			DeviceAccess: false,
		},
		Context:             libcontainer.Context{},
		RequiredDeviceNodes: nodes.DefaultNodes,
		OptionalDeviceNodes: []string{"fuse"},
	}
	if apparmor.IsEnabled() {
		container.Context["apparmor_profile"] = "docker-default"
	}
	return container
}
