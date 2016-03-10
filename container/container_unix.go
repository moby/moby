// +build linux freebsd

package container

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/opencontainers/runc/libcontainer/label"
)

// DefaultSHMSize is the default size (64MB) of the SHM which will be mounted in the container
const DefaultSHMSize int64 = 67108864

// Container holds the fields specific to unixen implementations.
// See CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
	AppArmorProfile string
	HostnamePath    string
	HostsPath       string
	ShmPath         string
	ResolvConfPath  string
	SeccompProfile  string
	NoNewPrivileges bool
}

// CreateDaemonEnvironment returns the list of all environment variables given the list of
// environment variables related to links.
// Sets PATH, HOSTNAME and if container.Config.Tty is set: TERM.
// The defaults set here do not override the values in container.Config.Env
func (container *Container) CreateDaemonEnvironment(linkedEnv []string) []string {
	// if a domain name was specified, append it to the hostname (see #7851)
	fullHostname := container.Config.Hostname
	if container.Config.Domainname != "" {
		fullHostname = fmt.Sprintf("%s.%s", fullHostname, container.Config.Domainname)
	}
	// Setup environment
	env := []string{
		"PATH=" + system.DefaultPathEnv,
		"HOSTNAME=" + fullHostname,
	}
	if container.Config.Tty {
		env = append(env, "TERM=xterm")
	}
	env = append(env, linkedEnv...)
	// because the env on the container can override certain default values
	// we need to replace the 'env' keys where they match and append anything
	// else.
	env = utils.ReplaceOrAppendEnvValues(env, container.Config.Env)

	return env
}

// TrySetNetworkMount attempts to set the network mounts given a provided destination and
// the path to use for it; return true if the given destination was a network mount file
func (container *Container) TrySetNetworkMount(destination string, path string) bool {
	if destination == "/etc/resolv.conf" {
		container.ResolvConfPath = path
		return true
	}
	if destination == "/etc/hostname" {
		container.HostnamePath = path
		return true
	}
	if destination == "/etc/hosts" {
		container.HostsPath = path
		return true
	}

	return false
}

// BuildHostnameFile writes the container's hostname file.
func (container *Container) BuildHostnameFile() error {
	hostnamePath, err := container.GetRootResourcePath("hostname")
	if err != nil {
		return err
	}
	container.HostnamePath = hostnamePath

	if container.Config.Domainname != "" {
		return ioutil.WriteFile(container.HostnamePath, []byte(fmt.Sprintf("%s.%s\n", container.Config.Hostname, container.Config.Domainname)), 0644)
	}
	return ioutil.WriteFile(container.HostnamePath, []byte(container.Config.Hostname+"\n"), 0644)
}

// appendNetworkMounts appends any network mounts to the array of mount points passed in
func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	for _, mnt := range container.NetworkMounts() {
		dest, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			return nil, err
		}
		volumeMounts = append(volumeMounts, volume.MountPoint{Destination: dest})
	}
	return volumeMounts, nil
}

// NetworkMounts returns the list of network mounts.
func (container *Container) NetworkMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	shared := container.HostConfig.NetworkMode.IsContainer()
	if container.ResolvConfPath != "" {
		if _, err := os.Stat(container.ResolvConfPath); err != nil {
			logrus.Warnf("ResolvConfPath set to %q, but can't stat this filename (err = %v); skipping", container.ResolvConfPath, err)
		} else {
			label.Relabel(container.ResolvConfPath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/resolv.conf"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.ResolvConfPath,
				Destination: "/etc/resolv.conf",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	if container.HostnamePath != "" {
		if _, err := os.Stat(container.HostnamePath); err != nil {
			logrus.Warnf("HostnamePath set to %q, but can't stat this filename (err = %v); skipping", container.HostnamePath, err)
		} else {
			label.Relabel(container.HostnamePath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hostname"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostnamePath,
				Destination: "/etc/hostname",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	if container.HostsPath != "" {
		if _, err := os.Stat(container.HostsPath); err != nil {
			logrus.Warnf("HostsPath set to %q, but can't stat this filename (err = %v); skipping", container.HostsPath, err)
		} else {
			label.Relabel(container.HostsPath, container.MountLabel, shared)
			writable := !container.HostConfig.ReadonlyRootfs
			if m, exists := container.MountPoints["/etc/hosts"]; exists {
				writable = m.RW
			}
			mounts = append(mounts, execdriver.Mount{
				Source:      container.HostsPath,
				Destination: "/etc/hosts",
				Writable:    writable,
				Propagation: volume.DefaultPropagationMode,
			})
		}
	}
	return mounts
}

// CopyImagePathContent copies files in destination to the volume.
func (container *Container) CopyImagePathContent(v volume.Volume, destination string) error {
	rootfs, err := symlink.FollowSymlinkInScope(filepath.Join(container.BaseFS, destination), container.BaseFS)
	if err != nil {
		return err
	}

	if _, err = ioutil.ReadDir(rootfs); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	path, err := v.Mount()
	if err != nil {
		return err
	}

	if err := copyExistingContents(rootfs, path); err != nil {
		return err
	}

	return v.Unmount()
}

// ShmResourcePath returns path to shm
func (container *Container) ShmResourcePath() (string, error) {
	return container.GetRootResourcePath("shm")
}

// HasMountFor checks if path is a mountpoint
func (container *Container) HasMountFor(path string) bool {
	_, exists := container.MountPoints[path]
	return exists
}

// UnmountIpcMounts uses the provided unmount function to unmount shm and mqueue if they were mounted
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
	if container.HostConfig.IpcMode.IsContainer() || container.HostConfig.IpcMode.IsHost() {
		return
	}

	var warnings []string

	if !container.HasMountFor("/dev/shm") {
		shmPath, err := container.ShmResourcePath()
		if err != nil {
			logrus.Error(err)
			warnings = append(warnings, err.Error())
		} else if shmPath != "" {
			if err := unmount(shmPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to umount %s: %v", shmPath, err))
			}

		}
	}

	if len(warnings) > 0 {
		logrus.Warnf("failed to cleanup ipc mounts:\n%v", strings.Join(warnings, "\n"))
	}
}

// IpcMounts returns the list of IPC mounts
func (container *Container) IpcMounts() []execdriver.Mount {
	var mounts []execdriver.Mount

	if !container.HasMountFor("/dev/shm") {
		label.SetFileLabel(container.ShmPath, container.MountLabel)
		mounts = append(mounts, execdriver.Mount{
			Source:      container.ShmPath,
			Destination: "/dev/shm",
			Writable:    true,
			Propagation: volume.DefaultPropagationMode,
		})
	}
	return mounts
}

func updateCommand(c *execdriver.Command, resources containertypes.Resources) {
	c.Resources.BlkioWeight = resources.BlkioWeight
	c.Resources.CPUShares = resources.CPUShares
	c.Resources.CPUPeriod = resources.CPUPeriod
	c.Resources.CPUQuota = resources.CPUQuota
	c.Resources.CpusetCpus = resources.CpusetCpus
	c.Resources.CpusetMems = resources.CpusetMems
	c.Resources.Memory = resources.Memory
	c.Resources.MemorySwap = resources.MemorySwap
	c.Resources.MemoryReservation = resources.MemoryReservation
	c.Resources.KernelMemory = resources.KernelMemory
}

// UpdateContainer updates configuration of a container.
func (container *Container) UpdateContainer(hostConfig *containertypes.HostConfig) error {
	container.Lock()

	// update resources of container
	resources := hostConfig.Resources
	cResources := &container.HostConfig.Resources
	if resources.BlkioWeight != 0 {
		cResources.BlkioWeight = resources.BlkioWeight
	}
	if resources.CPUShares != 0 {
		cResources.CPUShares = resources.CPUShares
	}
	if resources.CPUPeriod != 0 {
		cResources.CPUPeriod = resources.CPUPeriod
	}
	if resources.CPUQuota != 0 {
		cResources.CPUQuota = resources.CPUQuota
	}
	if resources.CpusetCpus != "" {
		cResources.CpusetCpus = resources.CpusetCpus
	}
	if resources.CpusetMems != "" {
		cResources.CpusetMems = resources.CpusetMems
	}
	if resources.Memory != 0 {
		cResources.Memory = resources.Memory
	}
	if resources.MemorySwap != 0 {
		cResources.MemorySwap = resources.MemorySwap
	}
	if resources.MemoryReservation != 0 {
		cResources.MemoryReservation = resources.MemoryReservation
	}
	if resources.KernelMemory != 0 {
		cResources.KernelMemory = resources.KernelMemory
	}

	// update HostConfig of container
	if hostConfig.RestartPolicy.Name != "" {
		container.HostConfig.RestartPolicy = hostConfig.RestartPolicy
	}
	container.Unlock()

	// If container is not running, update hostConfig struct is enough,
	// resources will be updated when the container is started again.
	// If container is running (including paused), we need to update
	// the command so we can update configs to the real world.
	if container.IsRunning() {
		container.Lock()
		updateCommand(container.Command, *cResources)
		container.Unlock()
	}

	if err := container.ToDiskLocking(); err != nil {
		logrus.Errorf("Error saving updated container: %v", err)
		return err
	}

	return nil
}

func detachMounted(path string) error {
	return syscall.Unmount(path, syscall.MNT_DETACH)
}

// UnmountVolumes unmounts all volumes
func (container *Container) UnmountVolumes(forceSyscall bool, volumeEventLog func(name, action string, attributes map[string]string)) error {
	var (
		volumeMounts []volume.MountPoint
		err          error
	)

	for _, mntPoint := range container.MountPoints {
		dest, err := container.GetResourcePath(mntPoint.Destination)
		if err != nil {
			return err
		}

		volumeMounts = append(volumeMounts, volume.MountPoint{Destination: dest, Volume: mntPoint.Volume})
	}

	// Append any network mounts to the list (this is a no-op on Windows)
	if volumeMounts, err = appendNetworkMounts(container, volumeMounts); err != nil {
		return err
	}

	for _, volumeMount := range volumeMounts {
		if forceSyscall {
			if err := detachMounted(volumeMount.Destination); err != nil {
				logrus.Warnf("%s unmountVolumes: Failed to do lazy umount %v", container.ID, err)
			}
		}

		if volumeMount.Volume != nil {
			if err := volumeMount.Volume.Unmount(); err != nil {
				return err
			}

			attributes := map[string]string{
				"driver":    volumeMount.Volume.DriverName(),
				"container": container.ID,
			}
			volumeEventLog(volumeMount.Volume.Name(), "unmount", attributes)
		}
	}

	return nil
}

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}
		if len(srcList) == 0 {
			// If the source volume is empty, copies files from the root into the volume
			if err := chrootarchive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}
	return copyOwnership(source, destination)
}

// copyOwnership copies the permissions and uid:gid of the source file
// to the destination file
func copyOwnership(source, destination string) error {
	stat, err := system.Stat(source)
	if err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.UID()), int(stat.GID())); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode()))
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	for dest, data := range container.HostConfig.Tmpfs {
		mounts = append(mounts, execdriver.Mount{
			Source:      "tmpfs",
			Destination: dest,
			Data:        data,
		})
	}
	return mounts
}

// cleanResourcePath cleans a resource path and prepares to combine with mnt path
func cleanResourcePath(path string) string {
	return filepath.Join(string(os.PathSeparator), path)
}

// canMountFS determines if the file system for the container
// can be mounted locally. A no-op on non-Windows platforms
func (container *Container) canMountFS() bool {
	return true
}
