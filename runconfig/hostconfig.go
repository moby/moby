package runconfig

import (
	"strings"

	"github.com/docker/docker/engine"
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

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	LxcConf         []KeyValuePair
	Memory          int64  // Memory limit (in bytes)
	MemorySwap      int64  // Total memory usage (memory + swap); set `-1` to disable swap
	CpuShares       int64  // CPU shares (relative weight vs. other containers)
	CpusetCpus      string // CpusetCpus 0-2, 0,1
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

// This is used by the create command when you want to set both the
// Config and the HostConfig in the same call
type ConfigAndHostConfig struct {
	Config
	HostConfig HostConfig
}

func MergeConfigs(config *Config, hostConfig *HostConfig) *ConfigAndHostConfig {
	return &ConfigAndHostConfig{
		*config,
		*hostConfig,
	}
}

func ContainerHostConfigFromJob(env *engine.Env) *HostConfig {
	if env.Exists("HostConfig") {
		hostConfig := HostConfig{}
		env.GetJson("HostConfig", &hostConfig)

		// FIXME: These are for backward compatibility, if people use these
		// options with `HostConfig`, we should still make them workable.
		if env.Exists("Memory") && hostConfig.Memory == 0 {
			hostConfig.Memory = env.GetInt64("Memory")
		}
		if env.Exists("MemorySwap") && hostConfig.MemorySwap == 0 {
			hostConfig.MemorySwap = env.GetInt64("MemorySwap")
		}
		if env.Exists("CpuShares") && hostConfig.CpuShares == 0 {
			hostConfig.CpuShares = env.GetInt64("CpuShares")
		}
		if env.Exists("Cpuset") && hostConfig.CpusetCpus == "" {
			hostConfig.CpusetCpus = env.Get("Cpuset")
		}

		return &hostConfig
	}

	hostConfig := &HostConfig{
		ContainerIDFile: env.Get("ContainerIDFile"),
		Memory:          env.GetInt64("Memory"),
		MemorySwap:      env.GetInt64("MemorySwap"),
		CpuShares:       env.GetInt64("CpuShares"),
		CpusetCpus:      env.Get("CpusetCpus"),
		Privileged:      env.GetBool("Privileged"),
		PublishAllPorts: env.GetBool("PublishAllPorts"),
		NetworkMode:     NetworkMode(env.Get("NetworkMode")),
		IpcMode:         IpcMode(env.Get("IpcMode")),
		PidMode:         PidMode(env.Get("PidMode")),
		ReadonlyRootfs:  env.GetBool("ReadonlyRootfs"),
		CgroupParent:    env.Get("CgroupParent"),
	}

	// FIXME: This is for backward compatibility, if people use `Cpuset`
	// in json, make it workable, we will only pass hostConfig.CpusetCpus
	// to execDriver.
	if env.Exists("Cpuset") && hostConfig.CpusetCpus == "" {
		hostConfig.CpusetCpus = env.Get("Cpuset")
	}

	env.GetJson("LxcConf", &hostConfig.LxcConf)
	env.GetJson("PortBindings", &hostConfig.PortBindings)
	env.GetJson("Devices", &hostConfig.Devices)
	env.GetJson("RestartPolicy", &hostConfig.RestartPolicy)
	env.GetJson("Ulimits", &hostConfig.Ulimits)
	env.GetJson("LogConfig", &hostConfig.LogConfig)
	hostConfig.SecurityOpt = env.GetList("SecurityOpt")
	if Binds := env.GetList("Binds"); Binds != nil {
		hostConfig.Binds = Binds
	}
	if Links := env.GetList("Links"); Links != nil {
		hostConfig.Links = Links
	}
	if Dns := env.GetList("Dns"); Dns != nil {
		hostConfig.Dns = Dns
	}
	if DnsSearch := env.GetList("DnsSearch"); DnsSearch != nil {
		hostConfig.DnsSearch = DnsSearch
	}
	if ExtraHosts := env.GetList("ExtraHosts"); ExtraHosts != nil {
		hostConfig.ExtraHosts = ExtraHosts
	}
	if VolumesFrom := env.GetList("VolumesFrom"); VolumesFrom != nil {
		hostConfig.VolumesFrom = VolumesFrom
	}
	if CapAdd := env.GetList("CapAdd"); CapAdd != nil {
		hostConfig.CapAdd = CapAdd
	}
	if CapDrop := env.GetList("CapDrop"); CapDrop != nil {
		hostConfig.CapDrop = CapDrop
	}

	return hostConfig
}
