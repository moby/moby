package native

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/cgroups"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/runtime/execdriver"
	"os"
	"strings"
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

	loopbackNetwork := libcontainer.Network{
		Mtu:     c.Network.Mtu,
		Address: fmt.Sprintf("%s/%d", "127.0.0.1", 0),
		Gateway: "localhost",
		Type:    "loopback",
		Context: libcontainer.Context{},
	}

	container.Networks = []*libcontainer.Network{
		&loopbackNetwork,
	}

	if c.Network.Interface != nil {
		vethNetwork := libcontainer.Network{
			Mtu:     c.Network.Mtu,
			Address: fmt.Sprintf("%s/%d", c.Network.Interface.IPAddress, c.Network.Interface.IPPrefixLen),
			Gateway: c.Network.Interface.Gateway,
			Type:    "veth",
			Context: libcontainer.Context{
				"prefix": "veth",
				"bridge": c.Network.Interface.Bridge,
			},
		}
		container.Networks = append(container.Networks, &vethNetwork)
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

	configureCustomOptions(container, c.Config["native"])

	return container
}

// configureCustomOptions takes string commands from the user and allows modification of the
// container's default configuration.
//
// format: <key> <value>
// i.e: cap +MKNOD cap -NET_ADMIN
// i.e: cgroup devices.allow *:*
func configureCustomOptions(container *libcontainer.Container, opts []string) {
	for _, opt := range opts {
		var (
			parts = strings.Split(strings.TrimSpace(opt), " ")
			value = strings.TrimSpace(parts[1])
		)
		switch parts[0] {
		case "cap":
			c := container.CapabilitiesMask.Get(value[1:])
			if c == nil {
				continue
			}
			switch value[0] {
			case '-':
				c.Enabled = false
			case '+':
				c.Enabled = true
			default:
				// do error here
			}
		case "ns":
			ns := container.Namespaces.Get(value[1:])
			switch value[0] {
			case '-':
				ns.Enabled = false
			case '+':
				ns.Enabled = true
			default:
				// error
			}
		}
	}
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
