package container

import (
	"strings"

	"github.com/docker/engine-api/types/blkiodev"
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
)

// NetworkMode represents the container network stack.
type NetworkMode string

// IsolationLevel represents the isolation level of a container. The supported
// values are platform specific
type IsolationLevel string

// IsDefault indicates the default isolation level of a container. On Linux this
// is the native driver. On Windows, this is a Windows Server Container.
func (i IsolationLevel) IsDefault() bool {
	return strings.ToLower(string(i)) == "default" || string(i) == ""
}

// IpcMode represents the container ipc stack.
type IpcMode string

// IsPrivate indicates whether the container uses it's private ipc stack.
func (n IpcMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsHost indicates whether the container uses the host's ipc stack.
func (n IpcMode) IsHost() bool {
	return n == "host"
}

// IsContainer indicates whether the container uses a container's ipc stack.
func (n IpcMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// Valid indicates whether the ipc stack is valid.
func (n IpcMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	case "container":
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// Container returns the name of the container ipc stack is going to be used.
func (n IpcMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// UTSMode represents the UTS namespace of the container.
type UTSMode string

// IsPrivate indicates whether the container uses it's private UTS namespace.
func (n UTSMode) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's UTS namespace.
func (n UTSMode) IsHost() bool {
	return n == "host"
}

// Valid indicates whether the UTS namespace is valid.
func (n UTSMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

// PidMode represents the pid stack of the container.
type PidMode string

// IsPrivate indicates whether the container uses it's private pid stack.
func (n PidMode) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's pid stack.
func (n PidMode) IsHost() bool {
	return n == "host"
}

// Valid indicates whether the pid stack is valid.
func (n PidMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

// DeviceMapping represents the device mapping between the host and the container.
type DeviceMapping struct {
	PathOnHost        string `json:",omitempty"`
	PathInContainer   string `json:",omitempty"`
	CgroupPermissions string `json:",omitempty"`
}

// RestartPolicy represents the restart policies of the container.
type RestartPolicy struct {
	Name              string `json:",omitempty"`
	MaximumRetryCount int    `json:",omitempty"`
}

// IsNone indicates whether the container has the "no" restart policy.
// This means the container will not automatically restart when exiting.
func (rp *RestartPolicy) IsNone() bool {
	return rp.Name == "no"
}

// IsAlways indicates whether the container has the "always" restart policy.
// This means the container will automatically restart regardless of the exit status.
func (rp *RestartPolicy) IsAlways() bool {
	return rp.Name == "always"
}

// IsOnFailure indicates whether the container has the "on-failure" restart policy.
// This means the contain will automatically restart of exiting with a non-zero exit status.
func (rp *RestartPolicy) IsOnFailure() bool {
	return rp.Name == "on-failure"
}

// IsUnlessStopped indicates whether the container has the
// "unless-stopped" restart policy. This means the container will
// automatically restart unless user has put it to stopped state.
func (rp *RestartPolicy) IsUnlessStopped() bool {
	return rp.Name == "unless-stopped"
}

// LogConfig represents the logging configuration of the container.
type LogConfig struct {
	Type   string            `json:",omitempty"`
	Config map[string]string `json:",omitempty"`
}

// Resources contains container's resources (cgroups config, ulimits...)
type Resources struct {
	// Applicable to all platforms
	CPUShares int64 `json:"CpuShares,omitempty"` // CPU shares (relative weight vs. other containers)

	// Applicable to UNIX platforms
	CgroupParent         string                     `json:",omitempty"` // Parent cgroup.
	BlkioWeight          uint16                     `json:",omitempty"` // Block IO weight (relative weight vs. other containers)
	BlkioWeightDevice    []*blkiodev.WeightDevice   `json:",omitempty"`
	BlkioDeviceReadBps   []*blkiodev.ThrottleDevice `json:",omitempty"`
	BlkioDeviceWriteBps  []*blkiodev.ThrottleDevice `json:",omitempty"`
	BlkioDeviceReadIOps  []*blkiodev.ThrottleDevice `json:",omitempty"`
	BlkioDeviceWriteIOps []*blkiodev.ThrottleDevice `json:",omitempty"`
	CPUPeriod            int64                      `json:"CpuPeriod,omitempty"` // CPU CFS (Completely Fair Scheduler) period
	CPUQuota             int64                      `json:"CpuQuota,omitempty"`  // CPU CFS (Completely Fair Scheduler) quota
	CpusetCpus           string                     `json:",omitempty"`          // CpusetCpus 0-2, 0,1
	CpusetMems           string                     `json:",omitempty"`          // CpusetMems 0-2, 0,1
	Devices              []DeviceMapping            `json:",omitempty"`          // List of devices to map inside the container
	KernelMemory         int64                      `json:",omitempty"`          // Kernel memory limit (in bytes)
	Memory               int64                      `json:",omitempty"`          // Memory limit (in bytes)
	MemoryReservation    int64                      `json:",omitempty"`          // Memory soft limit (in bytes)
	MemorySwap           int64                      `json:",omitempty"`          // Total memory usage (memory + swap); set `-1` to disable swap
	MemorySwappiness     *int64                     `json:",omitempty"`          // Tuning container memory swappiness behaviour
	OomKillDisable       *bool                      `json:",omitempty"`          // Whether to disable OOM Killer or not
	PidsLimit            int64                      `json:",omitempty"`          // Setting pids limit for a container
	Ulimits              []*units.Ulimit            `json:",omitempty"`          // List of ulimits to be set in the container
}

// UpdateConfig holds the mutable attributes of a Container.
// Those attributes can be updated at runtime.
type UpdateConfig struct {
	// Contains container's resources (cgroups, ulimits)
	Resources `json:",omitempty"`
}

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig struct {
	// Applicable to all platforms
	Binds           []string      `json:",omitempty"` // List of volume bindings for this container
	ContainerIDFile string        `json:",omitempty"` // File (path) where the containerId is written
	LogConfig       LogConfig     `json:",omitempty"` // Configuration of the logs for this container
	NetworkMode     NetworkMode   `json:",omitempty"` // Network mode to use for the container
	PortBindings    nat.PortMap   `json:",omitempty"` // Port mapping between the exposed port (container) and the host
	RestartPolicy   RestartPolicy `json:",omitempty"` // Restart policy to be used for the container
	VolumeDriver    string        `json:",omitempty"` // Name of the volume driver used to mount volumes
	VolumesFrom     []string      `json:",omitempty"` // List of volumes to take from other container

	// Applicable to UNIX platforms
	CapAdd          *strslice.StrSlice `json:",omitempty"`           // List of kernel capabilities to add to the container
	CapDrop         *strslice.StrSlice `json:",omitempty"`           // List of kernel capabilities to remove from the container
	DNS             []string           `json:"Dns"`                  // List of DNS server to lookup
	DNSOptions      []string           `json:"DnsOptions,omitempty"` // List of DNSOption to look for
	DNSSearch       []string           `json:"DnsSearch,omitempty"`  // List of DNSSearch to look for
	ExtraHosts      []string           `json:",omitempty"`           // List of extra hosts
	GroupAdd        []string           `json:",omitempty"`           // List of additional groups that the container process will run as
	IpcMode         IpcMode            `json:",omitempty"`           // IPC namespace to use for the container
	Links           []string           `json:",omitempty"`           // List of links (in the name:alias form)
	OomScoreAdj     int                `json:",omitempty"`           // Container preference for OOM-killing
	PidMode         PidMode            `json:",omitempty"`           // PID namespace to use for the container
	Privileged      bool               `json:",omitempty"`           // Is the container in privileged mode
	PublishAllPorts bool               `json:",omitempty"`           // Should docker publish all exposed port for the container
	ReadonlyRootfs  bool               `json:",omitempty"`           // Is the container root filesystem in read-only
	SecurityOpt     []string           `json:",omitempty"`           // List of string values to customize labels for MLS systems, such as SELinux.
	StorageOpt      []string           `json:",omitempty"`           // Graph storage options per container
	Tmpfs           map[string]string  `json:",omitempty"`           // List of tmpfs (mounts) used for the container
	UTSMode         UTSMode            `json:",omitempty"`           // UTS namespace to use for the container
	ShmSize         int64              `json:",omitempty"`           // Total shm memory usage

	// Applicable to Windows
	ConsoleSize [2]int         `json:",omitempty"` // Initial console size
	Isolation   IsolationLevel `json:",omitempty"` // Isolation level of the container (eg default, hyperv)

	// Contains container's resources (cgroups, ulimits)
	Resources `json:",omitempty"`
}
