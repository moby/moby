// +build windows

package container

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/volume"
)

// Container holds fields specific to the Windows implementation. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
}

// CreateDaemonEnvironment creates a new environment variable slice for this container.
func (container *Container) CreateDaemonEnvironment(linkedEnv []string) []string {
	// On Windows, nothing to link. Just return the container environment.
	return container.Config.Env
}

// SetupWorkingDirectory initializes the container working directory.
// This is a NOOP In windows.
func (container *Container) SetupWorkingDirectory() error {
	return nil
}

// UnmountIpcMounts unmount Ipc related mounts.
// This is a NOOP on windows.
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
}

// IpcMounts returns the list of Ipc related mounts.
func (container *Container) IpcMounts() []execdriver.Mount {
	return nil
}

// UnmountVolumes explicitly unmounts volumes from the container.
func (container *Container) UnmountVolumes(forceSyscall bool) error {
	return nil
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() []execdriver.Mount {
	return nil
}

// UpdateContainer updates resources of a container
func (container *Container) UpdateContainer(hostConfig *container.HostConfig) error {
	return nil
}

// appendNetworkMounts appends any network mounts to the array of mount points passed in.
// Windows does not support network mounts (not to be confused with SMB network mounts), so
// this is a no-op.
func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	return volumeMounts, nil
}
