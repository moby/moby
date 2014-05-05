package native

import (
	"fmt"
	"os"

	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/execdriver/native/configuration"
	"github.com/dotcloud/docker/daemon/execdriver/native/template"
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func (d *driver) createContainer(c *execdriver.Command) (*libcontainer.Container, error) {
	container := template.New()

	container.Hostname = getEnv("HOSTNAME", c.Env)
	container.Tty = c.Tty
	container.User = c.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.Env
	container.Cgroups.Name = c.ID
	// check to see if we are running in ramdisk to disable pivot root
	container.NoPivotRoot = os.Getenv("DOCKER_RAMDISK") != ""
	container.Context["restrictions"] = "true"

	if err := d.createNetwork(container, c); err != nil {
		return nil, err
	}
	if c.Privileged {
		if err := d.setPrivileged(container); err != nil {
			return nil, err
		}
	} else {
		container.Mounts = append(container.Mounts, libcontainer.Mount{Type: "devtmpfs"})
	}
	if err := d.setupCgroups(container, c); err != nil {
		return nil, err
	}
	if err := d.setupMounts(container, c); err != nil {
		return nil, err
	}
	if err := d.setupLabels(container, c); err != nil {
		return nil, err
	}
	if err := configuration.ParseConfiguration(container, d.activeContainers, c.Config["native"]); err != nil {
		return nil, err
	}
	return container, nil
}

func (d *driver) createNetwork(container *libcontainer.Container, c *execdriver.Command) error {
	container.Networks = []*libcontainer.Network{
		{
			Mtu:     c.Network.Mtu,
			Address: fmt.Sprintf("%s/%d", "127.0.0.1", 0),
			Gateway: "localhost",
			Type:    "loopback",
			Context: libcontainer.Context{},
		},
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
	return nil
}

func (d *driver) setPrivileged(container *libcontainer.Container) error {
	for key := range container.CapabilitiesMask {
		container.CapabilitiesMask[key] = true
	}
	container.Cgroups.DeviceAccess = true

	delete(container.Context, "restrictions")

	if apparmor.IsEnabled() {
		container.Context["apparmor_profile"] = "unconfined"
	}
	return nil
}

func (d *driver) setupCgroups(container *libcontainer.Container, c *execdriver.Command) error {
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemoryReservation = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
	}
	return nil
}

func (d *driver) setupMounts(container *libcontainer.Container, c *execdriver.Command) error {
	for _, m := range c.Mounts {
		container.Mounts = append(container.Mounts, libcontainer.Mount{
			Type:        "bind",
			Source:      m.Source,
			Destination: m.Destination,
			Writable:    m.Writable,
			Private:     m.Private,
		})
	}
	return nil
}

func (d *driver) setupLabels(container *libcontainer.Container, c *execdriver.Command) error {
	container.Context["process_label"] = c.Config["process_label"][0]
	container.Context["mount_label"] = c.Config["mount_label"][0]
	return nil
}
