// +build linux,cgo

package native

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/mount"

	"github.com/docker/docker/volume"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
)

// createContainer populates and configures the container type with the
// data provided by the execdriver.Command
func (d *Driver) createContainer(c *execdriver.Command, hooks execdriver.Hooks) (container *configs.Config, err error) {
	container = execdriver.InitContainer(c)

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

		if c.SeccompProfile == "" {
			container.Seccomp = getDefaultSeccompProfile()
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

	if c.SeccompProfile != "" && c.SeccompProfile != "unconfined" {
		container.Seccomp, err = loadSeccompProfile(c.SeccompProfile)
		if err != nil {
			return nil, err
		}
	}

	if err := execdriver.SetupCgroups(container, c); err != nil {
		return nil, err
	}

	container.OomScoreAdj = c.OomScoreAdj

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

// If rootfs mount propagation is RPRIVATE, that means all the volumes are
// going to be private anyway. There is no need to apply per volume
// propagation on top. This is just an optimzation so that cost of per volume
// propagation is paid only if user decides to make some volume non-private
// which will force rootfs mount propagation to be non RPRIVATE.
func checkResetVolumePropagation(container *configs.Config) {
	if container.RootPropagation != mount.RPRIVATE {
		return
	}
	for _, m := range container.Mounts {
		m.PropagationFlags = nil
	}
}

func getMountInfo(mountinfo []*mount.Info, dir string) *mount.Info {
	for _, m := range mountinfo {
		if m.Mountpoint == dir {
			return m
		}
	}
	return nil
}

// Get the source mount point of directory passed in as argument. Also return
// optional fields.
func getSourceMount(source string) (string, string, error) {
	// Ensure any symlinks are resolved.
	sourcePath, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", "", err
	}

	mountinfos, err := mount.GetMounts()
	if err != nil {
		return "", "", err
	}

	mountinfo := getMountInfo(mountinfos, sourcePath)
	if mountinfo != nil {
		return sourcePath, mountinfo.Optional, nil
	}

	path := sourcePath
	for {
		path = filepath.Dir(path)

		mountinfo = getMountInfo(mountinfos, path)
		if mountinfo != nil {
			return path, mountinfo.Optional, nil
		}

		if path == "/" {
			break
		}
	}

	// If we are here, we did not find parent mount. Something is wrong.
	return "", "", fmt.Errorf("Could not find source mount of %s", source)
}

// Ensure mount point on which path is mouted, is shared.
func ensureShared(path string) error {
	sharedMount := false

	sourceMount, optionalOpts, err := getSourceMount(path)
	if err != nil {
		return err
	}
	// Make sure source mount point is shared.
	optsSplit := strings.Split(optionalOpts, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			sharedMount = true
			break
		}
	}

	if !sharedMount {
		return fmt.Errorf("Path %s is mounted on %s but it is not a shared mount.", path, sourceMount)
	}
	return nil
}

// Ensure mount point on which path is mounted, is either shared or slave.
func ensureSharedOrSlave(path string) error {
	sharedMount := false
	slaveMount := false

	sourceMount, optionalOpts, err := getSourceMount(path)
	if err != nil {
		return err
	}
	// Make sure source mount point is shared.
	optsSplit := strings.Split(optionalOpts, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			sharedMount = true
			break
		} else if strings.HasPrefix(opt, "master:") {
			slaveMount = true
			break
		}
	}

	if !sharedMount && !slaveMount {
		return fmt.Errorf("Path %s is mounted on %s but it is not a shared or slave mount.", path, sourceMount)
	}
	return nil
}

func (d *Driver) setupMounts(container *configs.Config, c *execdriver.Command) error {
	userMounts := make(map[string]struct{})
	for _, m := range c.Mounts {
		userMounts[m.Destination] = struct{}{}
	}

	// Filter out mounts that are overridden by user supplied mounts
	var defaultMounts []*configs.Mount
	_, mountDev := userMounts["/dev"]
	for _, m := range container.Mounts {
		if _, ok := userMounts[m.Destination]; !ok {
			if mountDev && strings.HasPrefix(m.Destination, "/dev/") {
				container.Devices = nil
				continue
			}
			defaultMounts = append(defaultMounts, m)
		}
	}
	container.Mounts = defaultMounts

	mountPropagationMap := map[string]int{
		"private":  mount.PRIVATE,
		"rprivate": mount.RPRIVATE,
		"shared":   mount.SHARED,
		"rshared":  mount.RSHARED,
		"slave":    mount.SLAVE,
		"rslave":   mount.RSLAVE,
	}

	for _, m := range c.Mounts {
		for _, cm := range container.Mounts {
			if cm.Destination == m.Destination {
				return derr.ErrorCodeMountDup.WithArgs(m.Destination)
			}
		}

		if m.Source == "tmpfs" {
			var (
				data  = "size=65536k"
				flags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
				err   error
			)
			fulldest := filepath.Join(c.Rootfs, m.Destination)
			if m.Data != "" {
				flags, data, err = mount.ParseTmpfsOptions(m.Data)
				if err != nil {
					return err
				}
			}
			container.Mounts = append(container.Mounts, &configs.Mount{
				Source:           m.Source,
				Destination:      m.Destination,
				Data:             data,
				Device:           "tmpfs",
				Flags:            flags,
				PremountCmds:     genTmpfsPremountCmd(c.TmpDir, fulldest, m.Destination),
				PostmountCmds:    genTmpfsPostmountCmd(c.TmpDir, fulldest, m.Destination),
				PropagationFlags: []int{mountPropagationMap[volume.DefaultPropagationMode]},
			})
			continue
		}
		flags := syscall.MS_BIND | syscall.MS_REC
		var pFlag int
		if !m.Writable {
			flags |= syscall.MS_RDONLY
		}

		// Determine property of RootPropagation based on volume
		// properties. If a volume is shared, then keep root propagtion
		// shared. This should work for slave and private volumes too.
		//
		// For slave volumes, it can be either [r]shared/[r]slave.
		//
		// For private volumes any root propagation value should work.

		pFlag = mountPropagationMap[m.Propagation]
		if pFlag == mount.SHARED || pFlag == mount.RSHARED {
			if err := ensureShared(m.Source); err != nil {
				return err
			}
			rootpg := container.RootPropagation
			if rootpg != mount.SHARED && rootpg != mount.RSHARED {
				execdriver.SetRootPropagation(container, mount.SHARED)
			}
		} else if pFlag == mount.SLAVE || pFlag == mount.RSLAVE {
			if err := ensureSharedOrSlave(m.Source); err != nil {
				return err
			}
			rootpg := container.RootPropagation
			if rootpg != mount.SHARED && rootpg != mount.RSHARED && rootpg != mount.SLAVE && rootpg != mount.RSLAVE {
				execdriver.SetRootPropagation(container, mount.RSLAVE)
			}
		}

		mount := &configs.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Device:      "bind",
			Flags:       flags,
		}

		if pFlag != 0 {
			mount.PropagationFlags = []int{pFlag}
		}

		container.Mounts = append(container.Mounts, mount)
	}

	checkResetVolumePropagation(container)
	return nil
}

func (d *Driver) setupLabels(container *configs.Config, c *execdriver.Command) {
	container.ProcessLabel = c.ProcessLabel
	container.MountLabel = c.MountLabel
}
