// +build solaris

package container

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/volume"
	"github.com/docker/engine-api/types/container"
)

// Container holds fields specific to the Solaris implementation. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// fields below here are platform specific.
	HostnamePath   string
	HostsPath      string
	ResolvConfPath string
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int
}

// CreateDaemonEnvironment creates a new environment variable slice for this container.
func (container *Container) CreateDaemonEnvironment(linkedEnv []string) []string {
	return nil
}

func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	return volumeMounts, nil
}

// TrySetNetworkMount attempts to set the network mounts given a provided destination and
// the path to use for it; return true if the given destination was a network mount file
func (container *Container) TrySetNetworkMount(destination string, path string) bool {
	return true
}

// NetworkMounts returns the list of network mounts.
func (container *Container) NetworkMounts() []Mount {
	var mount []Mount
	return mount
}

// CopyImagePathContent copies files in destination to the volume.
func (container *Container) CopyImagePathContent(v volume.Volume, destination string) error {
	return nil
}

// UnmountIpcMounts unmount Ipc related mounts.
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
}

// IpcMounts returns the list of Ipc related mounts.
func (container *Container) IpcMounts() []Mount {
	return nil
}

// UpdateContainer updates configuration of a container
func (container *Container) UpdateContainer(hostConfig *container.HostConfig) error {
	return nil
}

// UnmountVolumes explicitly unmounts volumes from the container.
func (container *Container) UnmountVolumes(forceSyscall bool, volumeEventLog func(name, action string, attributes map[string]string)) error {
	return nil
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() []Mount {
	var mounts []Mount
	return mounts
}

// cleanResourcePath cleans a resource path and prepares to combine with mnt path
func cleanResourcePath(path string) string {
	return filepath.Join(string(os.PathSeparator), path)
}

// BuildHostnameFile writes the container's hostname file.
func (container *Container) BuildHostnameFile() error {
	return nil
}

// canMountFS determines if the file system for the container
// can be mounted locally. A no-op on non-Windows platforms
func (container *Container) canMountFS() bool {
	return true
}
