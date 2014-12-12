// +build linux,cgo

package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/native/template"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/devices"
	"github.com/docker/libcontainer/mount"
	"github.com/docker/libcontainer/security/capabilities"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func (d *driver) createContainer(c *execdriver.Command) (*libcontainer.Config, error) {
	container := template.New()

	container.Hostname = getEnv("HOSTNAME", c.ProcessConfig.Env)
	container.Tty = c.ProcessConfig.Tty
	container.User = c.ProcessConfig.User
	container.WorkingDir = c.WorkingDir
	container.Env = c.ProcessConfig.Env
	container.Cgroups.Name = c.ID
	container.Cgroups.AllowedDevices = c.AllowedDevices
	container.MountConfig.DeviceNodes = c.AutoCreatedDevices
	container.RootFs = c.Rootfs

	// check to see if we are running in ramdisk to disable pivot root
	container.MountConfig.NoPivotRoot = os.Getenv("DOCKER_RAMDISK") != ""
	container.RestrictSys = true

	if err := d.createIpc(container, c); err != nil {
		return nil, err
	}

	if err := d.createNetwork(container, c); err != nil {
		return nil, err
	}

	if c.ProcessConfig.Privileged {
		if err := d.setPrivileged(container); err != nil {
			return nil, err
		}
	} else {
		if err := d.setCapabilities(container, c); err != nil {
			return nil, err
		}
	}

	if c.AppArmorProfile != "" {
		container.AppArmorProfile = c.AppArmorProfile
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
	d.Lock()
	for k, v := range d.activeContainers {
		cmds[k] = v.cmd
	}
	d.Unlock()

	return container, nil
}

func (d *driver) createNetwork(container *libcontainer.Config, c *execdriver.Command) error {
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
		},
	}

	if c.Network.Interface != nil {
		vethNetwork := libcontainer.Network{
			Mtu:        c.Network.Mtu,
			Address:    fmt.Sprintf("%s/%d", c.Network.Interface.IPAddress, c.Network.Interface.IPPrefixLen),
			MacAddress: c.Network.Interface.MacAddress,
			Gateway:    c.Network.Interface.Gateway,
			Type:       "veth",
			Bridge:     c.Network.Interface.Bridge,
			VethPrefix: "veth",
		}
		container.Networks = append(container.Networks, &vethNetwork)
	}

	if c.Network.ContainerID != "" {
		d.Lock()
		active := d.activeContainers[c.Network.ContainerID]
		d.Unlock()

		if active == nil || active.cmd.Process == nil {
			return fmt.Errorf("%s is not a valid running container to join", c.Network.ContainerID)
		}
		cmd := active.cmd

		nspath := filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "net")
		container.Networks = append(container.Networks, &libcontainer.Network{
			Type:   "netns",
			NsPath: nspath,
		})
	}

	return nil
}

func (d *driver) createIpc(container *libcontainer.Config, c *execdriver.Command) error {
	if c.Ipc.HostIpc {
		container.Namespaces["NEWIPC"] = false
		return nil
	}

	if c.Ipc.ContainerID != "" {
		d.Lock()
		active := d.activeContainers[c.Ipc.ContainerID]
		d.Unlock()

		if active == nil || active.cmd.Process == nil {
			return fmt.Errorf("%s is not a valid running container to join", c.Ipc.ContainerID)
		}
		cmd := active.cmd

		container.IpcNsPath = filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "ipc")
	}

	return nil
}

func (d *driver) setPrivileged(container *libcontainer.Config) (err error) {
	container.Capabilities = capabilities.GetAllCapabilities()
	container.Cgroups.AllowAllDevices = true

	hostDeviceNodes, err := devices.GetHostDeviceNodes()
	if err != nil {
		return err
	}
	container.MountConfig.DeviceNodes = hostDeviceNodes

	container.RestrictSys = false

	if apparmor.IsEnabled() {
		container.AppArmorProfile = "unconfined"
	}

	return nil
}

func (d *driver) setCapabilities(container *libcontainer.Config, c *execdriver.Command) (err error) {
	container.Capabilities, err = execdriver.TweakCapabilities(container.Capabilities, c.CapAdd, c.CapDrop)
	return err
}

func (d *driver) setupCgroups(container *libcontainer.Config, c *execdriver.Command) error {
	if c.Resources != nil {
		container.Cgroups.CpuShares = c.Resources.CpuShares
		container.Cgroups.Memory = c.Resources.Memory
		container.Cgroups.MemoryReservation = c.Resources.Memory
		container.Cgroups.MemorySwap = c.Resources.MemorySwap
		container.Cgroups.CpusetCpus = c.Resources.Cpuset
	}

	return nil
}

func (d *driver) setupMounts(container *libcontainer.Config, c *execdriver.Command) error {
	for _, m := range c.Mounts {
		container.MountConfig.Mounts = append(container.MountConfig.Mounts, &mount.Mount{
			Type:        "bind",
			Source:      m.Source,
			Destination: m.Destination,
			Writable:    m.Writable,
			Private:     m.Private,
			Slave:       m.Slave,
		})
	}

	return nil
}

func (d *driver) setupLabels(container *libcontainer.Config, c *execdriver.Command) error {
	container.ProcessLabel = c.ProcessLabel
	container.MountConfig.MountLabel = c.MountLabel

	return nil
}
