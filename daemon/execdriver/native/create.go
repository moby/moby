// +build linux,cgo

package native

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"

	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func (d *Driver) createContainer(c *execdriver.Command, hooks execdriver.Hooks) (*configs.Config, error) {
	container := execdriver.InitContainer(c)

	if err := d.createIpc(container, c); err != nil {
		return nil, err
	}

	if err := d.createPid(container, c); err != nil {
		return nil, err
	}

	if err := d.createUTS(container, c); err != nil {
		return nil, err
	}

	if err := d.setupRemappedRoot(container, c); err != nil {
		return nil, err
	}

	if err := d.createNetwork(container, c, hooks); err != nil {
		return nil, err
	}

	if c.ProcessConfig.Privileged {
		if !container.Readonlyfs {
			// clear readonly for /sys
			for i := range container.Mounts {
				if container.Mounts[i].Destination == "/sys" {
					container.Mounts[i].Flags &= ^syscall.MS_RDONLY
				}
			}
			container.ReadonlyPaths = nil
		}

		// clear readonly for cgroup
		for i := range container.Mounts {
			if container.Mounts[i].Device == "cgroup" {
				container.Mounts[i].Flags &= ^syscall.MS_RDONLY
			}
		}

		container.MaskPaths = nil
		if err := d.setPrivileged(container); err != nil {
			return nil, err
		}
	} else {
		if err := d.setCapabilities(container, c); err != nil {
			return nil, err
		}
	}
	// add CAP_ prefix to all caps for new libcontainer update to match
	// the spec format.
	for i, s := range container.Capabilities {
		if !strings.HasPrefix(s, "CAP_") {
			container.Capabilities[i] = fmt.Sprintf("CAP_%s", s)
		}
	}
	container.AdditionalGroups = c.GroupAdd

	if c.AppArmorProfile != "" {
		container.AppArmorProfile = c.AppArmorProfile
	}

	if err := execdriver.SetupCgroups(container, c); err != nil {
		return nil, err
	}

	if container.Readonlyfs {
		for i := range container.Mounts {
			switch container.Mounts[i].Destination {
			case "/proc", "/dev", "/dev/pts":
				continue
			}
			container.Mounts[i].Flags |= syscall.MS_RDONLY
		}

		/* These paths must be remounted as r/o */
		container.ReadonlyPaths = append(container.ReadonlyPaths, "/dev")
	}

	if err := d.setupMounts(container, c); err != nil {
		return nil, err
	}

	d.setupLabels(container, c)
	d.setupRlimits(container, c)
	return container, nil
}

func (d *Driver) createNetwork(container *configs.Config, c *execdriver.Command, hooks execdriver.Hooks) error {
	if c.Network == nil {
		return nil
	}
	if c.Network.ContainerID != "" {
		d.Lock()
		active := d.activeContainers[c.Network.ContainerID]
		d.Unlock()

		if active == nil {
			return fmt.Errorf("%s is not a valid running container to join", c.Network.ContainerID)
		}

		state, err := active.State()
		if err != nil {
			return err
		}

		container.Namespaces.Add(configs.NEWNET, state.NamespacePaths[configs.NEWNET])
		return nil
	}

	if c.Network.NamespacePath != "" {
		container.Namespaces.Add(configs.NEWNET, c.Network.NamespacePath)
		return nil
	}
	// only set up prestart hook if the namespace path is not set (this should be
	// all cases *except* for --net=host shared networking)
	container.Hooks = &configs.Hooks{
		Prestart: []configs.Hook{
			configs.NewFunctionHook(func(s configs.HookState) error {
				if len(hooks.PreStart) > 0 {
					for _, fnHook := range hooks.PreStart {
						// A closed channel for OOM is returned here as it will be
						// non-blocking and return the correct result when read.
						chOOM := make(chan struct{})
						close(chOOM)
						if err := fnHook(&c.ProcessConfig, s.Pid, chOOM); err != nil {
							return err
						}
					}
				}
				return nil
			}),
		},
	}
	return nil
}

func (d *Driver) createIpc(container *configs.Config, c *execdriver.Command) error {
	if c.Ipc.HostIpc {
		container.Namespaces.Remove(configs.NEWIPC)
		return nil
	}

	if c.Ipc.ContainerID != "" {
		d.Lock()
		active := d.activeContainers[c.Ipc.ContainerID]
		d.Unlock()

		if active == nil {
			return fmt.Errorf("%s is not a valid running container to join", c.Ipc.ContainerID)
		}

		state, err := active.State()
		if err != nil {
			return err
		}
		container.Namespaces.Add(configs.NEWIPC, state.NamespacePaths[configs.NEWIPC])
	}

	return nil
}

func (d *Driver) createPid(container *configs.Config, c *execdriver.Command) error {
	if c.Pid.HostPid {
		container.Namespaces.Remove(configs.NEWPID)
		return nil
	}

	return nil
}

func (d *Driver) createUTS(container *configs.Config, c *execdriver.Command) error {
	if c.UTS.HostUTS {
		container.Namespaces.Remove(configs.NEWUTS)
		container.Hostname = ""
		return nil
	}

	return nil
}

func (d *Driver) setupRemappedRoot(container *configs.Config, c *execdriver.Command) error {
	if c.RemappedRoot.UID == 0 {
		container.Namespaces.Remove(configs.NEWUSER)
		return nil
	}

	// convert the Docker daemon id map to the libcontainer variant of the same struct
	// this keeps us from having to import libcontainer code across Docker client + daemon packages
	cuidMaps := []configs.IDMap{}
	cgidMaps := []configs.IDMap{}
	for _, idMap := range c.UIDMapping {
		cuidMaps = append(cuidMaps, configs.IDMap(idMap))
	}
	for _, idMap := range c.GIDMapping {
		cgidMaps = append(cgidMaps, configs.IDMap(idMap))
	}
	container.UidMappings = cuidMaps
	container.GidMappings = cgidMaps

	for _, node := range container.Devices {
		node.Uid = uint32(c.RemappedRoot.UID)
		node.Gid = uint32(c.RemappedRoot.GID)
	}
	// TODO: until a kernel/mount solution exists for handling remount in a user namespace,
	// we must clear the readonly flag for the cgroups mount (@mrunalp concurs)
	for i := range container.Mounts {
		if container.Mounts[i].Device == "cgroup" {
			container.Mounts[i].Flags &= ^syscall.MS_RDONLY
		}
	}

	return nil
}

func (d *Driver) setPrivileged(container *configs.Config) (err error) {
	container.Capabilities = execdriver.GetAllCapabilities()
	container.Cgroups.AllowAllDevices = true

	hostDevices, err := devices.HostDevices()
	if err != nil {
		return err
	}
	container.Devices = hostDevices

	if apparmor.IsEnabled() {
		container.AppArmorProfile = "unconfined"
	}
	return nil
}

func (d *Driver) setCapabilities(container *configs.Config, c *execdriver.Command) (err error) {
	container.Capabilities, err = execdriver.TweakCapabilities(container.Capabilities, c.CapAdd, c.CapDrop)
	return err
}

func (d *Driver) setupRlimits(container *configs.Config, c *execdriver.Command) {
	if c.Resources == nil {
		return
	}

	for _, rlimit := range c.Resources.Rlimits {
		container.Rlimits = append(container.Rlimits, configs.Rlimit{
			Type: rlimit.Type,
			Hard: rlimit.Hard,
			Soft: rlimit.Soft,
		})
	}
}

func (d *Driver) setupMounts(container *configs.Config, c *execdriver.Command) error {
	userMounts := make(map[string]struct{})
	for _, m := range c.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	// Filter out mounts that are overriden by user supplied mounts
	var defaultMounts []*configs.Mount
	_, mountDev := userMounts["/dev"]
	for _, m := range container.Mounts {
		if _, ok := userMounts[m.Destination]; !ok {
			if mountDev && strings.HasPrefix(m.Destination, "/dev/") {
				continue
			}
			defaultMounts = append(defaultMounts, m)
		}
	}
	container.Mounts = defaultMounts

	for _, m := range c.Mounts {
		flags := syscall.MS_BIND | syscall.MS_REC
		if !m.Writable {
			flags |= syscall.MS_RDONLY
		}
		if m.Slave {
			flags |= syscall.MS_SLAVE
		}

		container.Mounts = append(container.Mounts, &configs.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Device:      "bind",
			Flags:       flags,
		})
	}
	return nil
}

func (d *Driver) setupLabels(container *configs.Config, c *execdriver.Command) {
	container.ProcessLabel = c.ProcessLabel
	container.MountLabel = c.MountLabel
}
