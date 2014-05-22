package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/execdriver/native/configuration"
	"github.com/dotcloud/docker/daemon/execdriver/native/template"
	"github.com/dotcloud/docker/pkg/apparmor"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/mount/nodes"
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
	cmds := make(map[string]*exec.Cmd)
	for k, v := range d.activeContainers {
		cmds[k] = v.cmd
	}
	if err := configuration.ParseConfiguration(container, cmds, c.Config["native"]); err != nil {
		return nil, err
	}
	return container, nil
}

func (d *driver) createNetwork(container *libcontainer.Container, c *execdriver.Command) error {
	if c.Network.HostNetworking {
		container.Namespaces["NEWNET"] = false
		return nil
	}
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

	if c.Network.ContainerID != "" {
		active := d.activeContainers[c.Network.ContainerID]
		if active == nil || active.cmd.Process == nil {
			return fmt.Errorf("%s is not a valid running container to join", c.Network.ContainerID)
		}
		cmd := active.cmd

		nspath := filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "net")
		container.Networks = append(container.Networks, &libcontainer.Network{
			Type: "netns",
			Context: libcontainer.Context{
				"nspath": nspath,
			},
		})
	}
	return nil
}

func (d *driver) setPrivileged(container *libcontainer.Container) (err error) {
	container.Capabilities = libcontainer.GetAllCapabilities()
	container.Cgroups.DeviceAccess = true

	delete(container.Context, "restrictions")

	container.OptionalDeviceNodes = nil
	if container.RequiredDeviceNodes, err = nodes.GetHostDeviceNodes(); err != nil {
		return err
	}

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
		container.Cgroups.CpusetCpus = c.Resources.Cpuset
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
