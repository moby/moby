package runconfig

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/nat"
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

func (n NetworkMode) IsBridge() bool {
	return n == "bridge"
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

type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

type LogConfig struct {
	Type   string
	Config map[string]string
}

// CommonHostConfig defines the host configuration which is common across
// platforms.
type CommonHostConfig struct {

	// TODO Windows. More fields may require factoring out here once the
	// capabilities of Windows Server containers are finalised. Have kept
	// properties which seem reasonable currently.

	Binds           []string
	ContainerIDFile string
	Memory          int64 // Memory limit (in bytes)
	MemorySwap      int64 // Total memory usage (memory + swap); set `-1` to disable swap
	CpuShares       int64 // CPU shares (relative weight vs. other containers)
	CpuPeriod       int64
	CpusetCpus      string // CpusetCpus 0-2, 0,1
	CpusetMems      string // CpusetMems 0-2, 0,1
	CpuQuota        int64
	PortBindings    nat.PortMap
	Links           []string
	PublishAllPorts bool
	Dns             []string
	DnsSearch       []string
	NetworkMode     NetworkMode
	RestartPolicy   RestartPolicy
	LogConfig       LogConfig
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
