package native

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/runtime/execdriver"
	"github.com/dotcloud/docker/runtime/execdriver/native/configuration"
	"os"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func (d *driver) createContainer(c *execdriver.Command) (*libcontainer.Container, error) {
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

	if err := configuration.ParseConfiguration(container, d.activeContainers, c.Config["native"]); err != nil {
		return nil, err
	}
	return container, nil
}
