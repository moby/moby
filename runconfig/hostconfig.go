package runconfig

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/ulimit"
)

// KeyValuePair is a structure that hold a value for a key.
type KeyValuePair struct {
	Key   string
	Value string
}

// NetworkMode represents the container network stack.
type NetworkMode string

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
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

// RestartPolicy represents the restart policies of the container.
type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
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
	Type   string
	Config map[string]string
}

// LxcConfig represents the specific LXC configuration of the container.
type LxcConfig struct {
	values []KeyValuePair
}

// MarshalJSON marshals (or serializes) the LxcConfig into JSON.
func (c *LxcConfig) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte{}, nil
	}
	return json.Marshal(c.Slice())
}

// UnmarshalJSON unmarshals (or deserializes) the specified byte slices from JSON to
// a LxcConfig.
func (c *LxcConfig) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var kv []KeyValuePair
	if err := json.Unmarshal(b, &kv); err != nil {
		var h map[string]string
		if err := json.Unmarshal(b, &h); err != nil {
			return err
		}
		for k, v := range h {
			kv = append(kv, KeyValuePair{k, v})
		}
	}
	c.values = kv

	return nil
}

// Len returns the number of specific lxc configuration.
func (c *LxcConfig) Len() int {
	if c == nil {
		return 0
	}
	return len(c.values)
}

// Slice returns the specific lxc configuration into a slice of KeyValuePair.
func (c *LxcConfig) Slice() []KeyValuePair {
	if c == nil {
		return nil
	}
	return c.values
}

// NewLxcConfig creates a LxcConfig from the specified slice of KeyValuePair.
func NewLxcConfig(values []KeyValuePair) *LxcConfig {
	return &LxcConfig{values}
}

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig struct {
	Binds             []string              // List of volume bindings for this container
	ContainerIDFile   string                // File (path) where the containerId is written
	LxcConf           *LxcConfig            // Additional lxc configuration
	Memory            int64                 // Memory limit (in bytes)
	MemoryReservation int64                 // Memory soft limit (in bytes)
	MemorySwap        int64                 // Total memory usage (memory + swap); set `-1` to disable swap
	KernelMemory      int64                 // Kernel memory limit (in bytes)
	CPUShares         int64                 `json:"CpuShares"` // CPU shares (relative weight vs. other containers)
	CPUPeriod         int64                 `json:"CpuPeriod"` // CPU CFS (Completely Fair Scheduler) period
	CpusetCpus        string                // CpusetCpus 0-2, 0,1
	CpusetMems        string                // CpusetMems 0-2, 0,1
	CPUQuota          int64                 `json:"CpuQuota"` // CPU CFS (Completely Fair Scheduler) quota
	BlkioWeight       uint16                // Block IO weight (relative weight vs. other containers)
	OomKillDisable    bool                  // Whether to disable OOM Killer or not
	MemorySwappiness  *int64                // Tuning container memory swappiness behaviour
	Privileged        bool                  // Is the container in privileged mode
	PortBindings      nat.PortMap           // Port mapping between the exposed port (container) and the host
	Links             []string              // List of links (in the name:alias form)
	PublishAllPorts   bool                  // Should docker publish all exposed port for the container
	DNS               []string              `json:"Dns"`        // List of DNS server to lookup
	DNSOptions        []string              `json:"DnsOptions"` // List of DNSOption to look for
	DNSSearch         []string              `json:"DnsSearch"`  // List of DNSSearch to look for
	ExtraHosts        []string              // List of extra hosts
	VolumesFrom       []string              // List of volumes to take from other container
	Devices           []DeviceMapping       // List of devices to map inside the container
	NetworkMode       NetworkMode           // Network namespace to use for the container
	IpcMode           IpcMode               // IPC namespace to use for the container
	PidMode           PidMode               // PID namespace to use for the container
	UTSMode           UTSMode               // UTS namespace to use for the container
	CapAdd            *stringutils.StrSlice // List of kernel capabilities to add to the container
	CapDrop           *stringutils.StrSlice // List of kernel capabilities to remove from the container
	GroupAdd          []string              // List of additional groups that the container process will run as
	RestartPolicy     RestartPolicy         // Restart policy to be used for the container
	SecurityOpt       []string              // List of string values to customize labels for MLS systems, such as SELinux.
	ReadonlyRootfs    bool                  // Is the container root filesystem in read-only
	Ulimits           []*ulimit.Ulimit      // List of ulimits to be set in the container
	LogConfig         LogConfig             // Configuration of the logs for this container
	CgroupParent      string                // Parent cgroup.
	ConsoleSize       [2]int                // Initial console size on Windows
	VolumeDriver      string                // Name of the volume driver used to mount volumes
}

// DecodeHostConfig creates a HostConfig based on the specified Reader.
// It assumes the content of the reader will be JSON, and decodes it.
func DecodeHostConfig(src io.Reader) (*HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w ContainerConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, err
	}

	hc := w.getHostConfig()
	return hc, nil
}

// SetDefaultNetModeIfBlank changes the NetworkMode in a HostConfig structure
// to default if it is not populated. This ensures backwards compatibility after
// the validation of the network mode was moved from the docker CLI to the
// docker daemon.
func SetDefaultNetModeIfBlank(hc *HostConfig) *HostConfig {
	if hc != nil {
		if hc.NetworkMode == NetworkMode("") {
			hc.NetworkMode = NetworkMode("default")
		}
	}
	return hc
}
