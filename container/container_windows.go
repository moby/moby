// +build windows

package container

import (
	"fmt"
	"os"
	"path/filepath"

	containertypes "github.com/docker/docker/api/types/container"
)

// Container holds fields specific to the Windows implementation. See
// CommonContainer for standard fields common to all containers.
type Container struct {
	CommonContainer

	// Fields below here are platform specific.
	NetworkSharedContainerID string
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int
}

// CreateDaemonEnvironment creates a new environment variable slice for this container.
func (container *Container) CreateDaemonEnvironment(_ bool, linkedEnv []string) []string {
	// because the env on the container can override certain default values
	// we need to replace the 'env' keys where they match and append anything
	// else.
	return ReplaceOrAppendEnvValues(linkedEnv, container.Config.Env)
}

// UnmountIpcMounts unmounts Ipc related mounts.
// This is a NOOP on windows.
func (container *Container) UnmountIpcMounts(unmount func(pth string) error) {
}

// IpcMounts returns the list of Ipc related mounts.
func (container *Container) IpcMounts() []Mount {
	return nil
}

// SecretMount returns the mount for the secret path
func (container *Container) SecretMount() *Mount {
	return nil
}

// UnmountSecrets unmounts the fs for secrets
func (container *Container) UnmountSecrets() error {
	return nil
}

// DetachAndUnmount unmounts all volumes.
// On Windows it only delegates to `UnmountVolumes` since there is nothing to
// force unmount.
func (container *Container) DetachAndUnmount(volumeEventLog func(name, action string, attributes map[string]string)) error {
	return container.UnmountVolumes(volumeEventLog)
}

// TmpfsMounts returns the list of tmpfs mounts
func (container *Container) TmpfsMounts() ([]Mount, error) {
	var mounts []Mount
	return mounts, nil
}

// UpdateContainer updates configuration of a container
func (container *Container) UpdateContainer(hostConfig *containertypes.HostConfig) error {
	container.Lock()
	defer container.Unlock()

	resources := hostConfig.Resources
	if resources.CPUShares != 0 ||
		resources.Memory != 0 ||
		resources.NanoCPUs != 0 ||
		resources.CgroupParent != "" ||
		resources.BlkioWeight != 0 ||
		len(resources.BlkioWeightDevice) != 0 ||
		len(resources.BlkioDeviceReadBps) != 0 ||
		len(resources.BlkioDeviceWriteBps) != 0 ||
		len(resources.BlkioDeviceReadIOps) != 0 ||
		len(resources.BlkioDeviceWriteIOps) != 0 ||
		resources.CPUPeriod != 0 ||
		resources.CPUQuota != 0 ||
		resources.CPURealtimePeriod != 0 ||
		resources.CPURealtimeRuntime != 0 ||
		resources.CpusetCpus != "" ||
		resources.CpusetMems != "" ||
		len(resources.Devices) != 0 ||
		len(resources.DeviceCgroupRules) != 0 ||
		resources.DiskQuota != 0 ||
		resources.KernelMemory != 0 ||
		resources.MemoryReservation != 0 ||
		resources.MemorySwap != 0 ||
		resources.MemorySwappiness != nil ||
		resources.OomKillDisable != nil ||
		resources.PidsLimit != 0 ||
		len(resources.Ulimits) != 0 ||
		resources.CPUCount != 0 ||
		resources.CPUPercent != 0 ||
		resources.IOMaximumIOps != 0 ||
		resources.IOMaximumBandwidth != 0 {
		return fmt.Errorf("resource updating isn't supported on Windows")
	}
	// update HostConfig of container
	if hostConfig.RestartPolicy.Name != "" {
		if container.HostConfig.AutoRemove && !hostConfig.RestartPolicy.IsNone() {
			return fmt.Errorf("Restart policy cannot be updated because AutoRemove is enabled for the container")
		}
		container.HostConfig.RestartPolicy = hostConfig.RestartPolicy
	}
	return nil
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

// EnableServiceDiscoveryOnDefaultNetwork Enable service discovery on default network
func (container *Container) EnableServiceDiscoveryOnDefaultNetwork() bool {
	return true
}
