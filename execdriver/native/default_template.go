package native

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func createContainer(c *execdriver.Command) *libcontainer.Container {
	container := getDefaultTemplate()

	container.Hostname = getEnv("HOSTNAME", c.Env)
	container.Tty = c.Tty
	container.User = c.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.Env

	if c.Network != nil {
		container.Networks = []*libcontainer.Network{
			{
				Mtu:     c.Network.Mtu,
				Address: fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen),
				Gateway: c.Network.Gateway,
				Type:    "veth",
				Context: libcontainer.Context{
					"prefix": "veth",
					"bridge": c.Network.Bridge,
				},
			},
		}
	}

	container.Cgroups.Name = c.ID
	if c.Privileged {
		container.CapabilitiesMask = nil
		container.Cgroups.DeviceAccess = true
		container.Context["apparmor_profile"] = "unconfined"
	}
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
	}
	// check to see if we are running in ramdisk to disable pivot root
	container.NoPivotRoot = os.Getenv("DOCKER_RAMDISK") != ""

	for _, m := range c.Mounts {
		container.Mounts = append(container.Mounts, libcontainer.Mount{m.Source, m.Destination, m.Writable, m.Private})
	}

	return container
}

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
