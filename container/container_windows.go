// +build windows

package container

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume"
	containertypes "github.com/docker/engine-api/types/container"
)

// Container holds fields specific to the Windows implementation. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	HostnamePath   string
	HostsPath      string
	ResolvConfPath string
	// Fields below here are platform specific.
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int
}

// CreateDaemonEnvironment creates a new environment variable slice for this container.
func (container *Container) CreateDaemonEnvironment(linkedEnv []string) []string {
	// because the env on the container can override certain default values
	// we need to replace the 'env' keys where they match and append anything
	// else.
	return utils.ReplaceOrAppendEnvValues(linkedEnv, container.Config.Env)
}

// UnmountIpcMounts unmount Ipc related mounts.
// This is a NOOP on windows.
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
}

// IpcMounts returns the list of Ipc related mounts.
func (container *Container) IpcMounts() []Mount {
	return nil
}

// UnmountVolumes explicitly unmounts volumes from the container.
func (container *Container) UnmountVolumes(forceSyscall bool, volumeEventLog func(name, action string, attributes map[string]string)) error {
	return nil
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() []Mount {
	return nil
}

// UpdateContainer updates configuration of a container
func (container *Container) UpdateContainer(hostConfig *containertypes.HostConfig) error {
	container.Lock()
	defer container.Unlock()
	resources := hostConfig.Resources
	if resources.BlkioWeight != 0 || resources.CPUShares != 0 ||
		resources.CPUPeriod != 0 || resources.CPUQuota != 0 ||
		resources.CpusetCpus != "" || resources.CpusetMems != "" ||
		resources.Memory != 0 || resources.MemorySwap != 0 ||
		resources.MemoryReservation != 0 || resources.KernelMemory != 0 {
		return fmt.Errorf("Resource updating isn't supported on Windows")
	}
	// update HostConfig of container
	if hostConfig.RestartPolicy.Name != "" {
		container.HostConfig.RestartPolicy = hostConfig.RestartPolicy
	}
	return nil
}

// appendNetworkMounts appends any network mounts to the array of mount points passed in.
// Windows does not support network mounts (not to be confused with SMB network mounts), so
// this is a no-op.
func appendNetworkMounts(container *Container, volumeMounts []volume.MountPoint) ([]volume.MountPoint, error) {
	return volumeMounts, nil
}

// cleanResourcePath cleans a resource path by removing C:\ syntax, and prepares
// to combine with a volume path
func cleanResourcePath(path string) string {
	if len(path) >= 2 {
		c := path[0]
		if path[1] == ':' && ('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
			path = path[2:]
		}
	}
	return filepath.Join(string(os.PathSeparator), path)
}

// BuildHostnameFile writes the container's hostname file.
func (container *Container) BuildHostnameFile() error {
	return nil
}

// canMountFS determines if the file system for the container
// can be mounted locally. In the case of Windows, this is not possible
// for Hyper-V containers during WORKDIR execution for example.
func (container *Container) canMountFS() bool {
	return !containertypes.Isolation.IsHyperV(container.HostConfig.Isolation)
}
