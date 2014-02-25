package native

import (
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// getDefaultTemplate returns the docker default for
// the libcontainer configuration file
func getDefaultTemplate() *libcontainer.Container {
	return &libcontainer.Container{
		Capabilities: libcontainer.Capabilities{
			libcontainer.CAP_SETPCAP,
			libcontainer.CAP_SYS_MODULE,
			libcontainer.CAP_SYS_RAWIO,
			libcontainer.CAP_SYS_PACCT,
			libcontainer.CAP_SYS_ADMIN,
			libcontainer.CAP_SYS_NICE,
			libcontainer.CAP_SYS_RESOURCE,
			libcontainer.CAP_SYS_TIME,
			libcontainer.CAP_SYS_TTY_CONFIG,
			libcontainer.CAP_MKNOD,
			libcontainer.CAP_AUDIT_WRITE,
			libcontainer.CAP_AUDIT_CONTROL,
			libcontainer.CAP_MAC_ADMIN,
			libcontainer.CAP_MAC_OVERRIDE,
			libcontainer.CAP_NET_ADMIN,
		},
		Namespaces: libcontainer.Namespaces{
			libcontainer.CLONE_NEWIPC,
			libcontainer.CLONE_NEWNET,
			libcontainer.CLONE_NEWNS,
			libcontainer.CLONE_NEWPID,
			libcontainer.CLONE_NEWUTS,
		},
		Cgroups: &cgroups.Cgroup{
			Parent:       "docker",
			DeviceAccess: false,
		},
	}
}
