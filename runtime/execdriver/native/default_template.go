package native

import (
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// getDefaultTemplate returns the docker default for
// the libcontainer configuration file
func getDefaultTemplate() *libcontainer.Container {
	return &libcontainer.Container{
		CapabilitiesMask: libcontainer.Capabilities{
			libcontainer.GetCapability("SETPCAP"),
			libcontainer.GetCapability("SYS_MODULE"),
			libcontainer.GetCapability("SYS_RAWIO"),
			libcontainer.GetCapability("SYS_PACCT"),
			libcontainer.GetCapability("SYS_ADMIN"),
			libcontainer.GetCapability("SYS_NICE"),
			libcontainer.GetCapability("SYS_RESOURCE"),
			libcontainer.GetCapability("SYS_TIME"),
			libcontainer.GetCapability("SYS_TTY_CONFIG"),
			libcontainer.GetCapability("MKNOD"),
			libcontainer.GetCapability("AUDIT_WRITE"),
			libcontainer.GetCapability("AUDIT_CONTROL"),
			libcontainer.GetCapability("MAC_OVERRIDE"),
			libcontainer.GetCapability("MAC_ADMIN"),
			libcontainer.GetCapability("NET_ADMIN"),
		},
		Namespaces: libcontainer.Namespaces{
			libcontainer.GetNamespace("NEWNS"),
			libcontainer.GetNamespace("NEWUTS"),
			libcontainer.GetNamespace("NEWIPC"),
			libcontainer.GetNamespace("NEWPID"),
			libcontainer.GetNamespace("NEWNET"),
		},
		Cgroups: &cgroups.Cgroup{
			Parent:       "docker",
			DeviceAccess: false,
		},
		Context: libcontainer.Context{
			"apparmor_profile": "docker-default",
		},
	}
}
