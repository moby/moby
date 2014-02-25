package native

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// createContainer populates and configrues the container type with the
// data provided by the execdriver.Command
func createContainer(c *execdriver.Command) *libcontainer.Container {
	container := getDefaultTemplate()

	container.Hostname = getEnv("HOSTNAME", c.Env)
	container.Tty = c.Tty
	container.User = c.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.Env

	if c.Network != nil {
		container.Network = &libcontainer.Network{
			Mtu:     c.Network.Mtu,
			Address: fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen),
			Gateway: c.Network.Gateway,
			Type:    "veth",
			Context: libcontainer.Context{
				"prefix": "dock",
				"bridge": c.Network.Bridge,
			},
		}
	}
	container.Cgroups.Name = c.ID
	if c.Privileged {
		container.Capabilities = nil
		container.Cgroups.DeviceAccess = true
	}
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
	}
	return container
}

// getDefaultTemplate returns the docker default for
// the libcontainer configuration file
func getDefaultTemplate() *libcontainer.Container {
	return &libcontainer.Container{
		Capabilities: libcontainer.Capabilities{
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
	}
}
