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
