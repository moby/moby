package runconfig

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/ulimit"
)

type KeyValuePair struct {
	Key   string
	Value string
}

type NetworkMode string

// IsPrivate indicates whether container use it's private network stack
func (n NetworkMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer() || n.IsNone())
}

func (n NetworkMode) IsHost() bool {
	return n == "host"
}

func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

func (n NetworkMode) IsNone() bool {
	return n == "none"
}

type IpcMode string

// IsPrivate indicates whether container use it's private ipc stack
func (n IpcMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

func (n IpcMode) IsHost() bool {
	return n == "host"
}

func (n IpcMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

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

func (n IpcMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

type PidMode string

// IsPrivate indicates whether container use it's private pid stack
func (n PidMode) IsPrivate() bool {
	return !(n.IsHost())
}

func (n PidMode) IsHost() bool {
	return n == "host"
}

func (n PidMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

type LogConfig struct {
	Type   string
	Config map[string]string
}

type LxcConfig struct {
	values []KeyValuePair
}

func (c *LxcConfig) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte{}, nil
	}
	return json.Marshal(c.Slice())
}

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

func (c *LxcConfig) Len() int {
	if c == nil {
		return 0
	}
	return len(c.values)
}

func (c *LxcConfig) Slice() []KeyValuePair {
	if c == nil {
		return nil
	}
	return c.values
}

func NewLxcConfig(values []KeyValuePair) *LxcConfig {
	return &LxcConfig{values}
}

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	LxcConf         *LxcConfig
	Memory          int64  // Memory limit (in bytes)
	MemorySwap      int64  // Total memory usage (memory + swap); set `-1` to disable swap
	CpuShares       int64  // CPU shares (relative weight vs. other containers)
	CpusetCpus      string // CpusetCpus 0-2, 0,1
	CpusetMems      string // CpusetMems 0-2, 0,1
	CpuQuota        int64
	Privileged      bool
	PortBindings    nat.PortMap
	Links           []string
	PublishAllPorts bool
	Dns             []string
	DnsSearch       []string
	ExtraHosts      []string
	VolumesFrom     []string
	Devices         []DeviceMapping
	NetworkMode     NetworkMode
	IpcMode         IpcMode
	PidMode         PidMode
	CapAdd          []string
	CapDrop         []string
	RestartPolicy   RestartPolicy
	SecurityOpt     []string
	ReadonlyRootfs  bool
	Ulimits         []*ulimit.Ulimit
	LogConfig       LogConfig
	CgroupParent    string // Parent cgroup.
}

func MergeConfigs(config *Config, hostConfig *HostConfig) *ContainerConfigWrapper {
	return &ContainerConfigWrapper{
		config,
		&hostConfigWrapper{InnerHostConfig: hostConfig},
	}
}

type hostConfigWrapper struct {
	InnerHostConfig *HostConfig `json:"HostConfig,omitempty"`
	Cpuset          string      `json:",omitempty"` // Deprecated. Exported for backwards compatibility.

	*HostConfig // Deprecated. Exported to read attrubutes from json that are not in the inner host config structure.
}

func (w hostConfigWrapper) GetHostConfig() *HostConfig {
	hc := w.HostConfig

	if hc == nil && w.InnerHostConfig != nil {
		hc = w.InnerHostConfig
	} else if w.InnerHostConfig != nil {
		if hc.Memory != 0 && w.InnerHostConfig.Memory == 0 {
			w.InnerHostConfig.Memory = hc.Memory
		}
		if hc.MemorySwap != 0 && w.InnerHostConfig.MemorySwap == 0 {
			w.InnerHostConfig.MemorySwap = hc.MemorySwap
		}
		if hc.CpuShares != 0 && w.InnerHostConfig.CpuShares == 0 {
			w.InnerHostConfig.CpuShares = hc.CpuShares
		}

		hc = w.InnerHostConfig
	}

	if hc != nil && w.Cpuset != "" && hc.CpusetCpus == "" {
		hc.CpusetCpus = w.Cpuset
	}

	return hc
}

func DecodeHostConfig(src io.Reader) (*HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w hostConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, err
	}

	hc := w.GetHostConfig()

	return hc, nil
}
