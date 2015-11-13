// +build windows

package container

import (
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libnetwork"
)

// DefaultPathEnv is deliberately empty on Windows as the default path will be set by
// the container. Docker has no context of what the default path should be.
const DefaultPathEnv = ""

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

// DisconnectFromNetwork disconnects a container from the network.
func (container *Container) DisconnectFromNetwork(n libnetwork.Network) error {
	return nil
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

// UnmountVolumes explicitely unmounts volumes from the container.
func (container *Container) UnmountVolumes(forceSyscall bool) error {
	return nil
}
