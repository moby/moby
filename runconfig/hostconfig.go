package runconfig

import (
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/utils"
)

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
	LxcConf         []utils.KeyValuePair
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

func ContainerHostConfigFromJob(job *engine.Job) *HostConfig {
	if job.EnvExists("HostConfig") {
		hostConfig := HostConfig{}
		job.GetenvJson("HostConfig", &hostConfig)

		// FIXME: These are for backward compatibility, if people use these
		// options with `HostConfig`, we should still make them workable.
		if job.EnvExists("Memory") && hostConfig.Memory == 0 {
			hostConfig.Memory = job.GetenvInt64("Memory")
		}
		if job.EnvExists("MemorySwap") && hostConfig.MemorySwap == 0 {
			hostConfig.MemorySwap = job.GetenvInt64("MemorySwap")
		}
		if job.EnvExists("CpuShares") && hostConfig.CpuShares == 0 {
			hostConfig.CpuShares = job.GetenvInt64("CpuShares")
		}
		if job.EnvExists("Cpuset") && hostConfig.CpusetCpus == "" {
			hostConfig.CpusetCpus = job.Getenv("Cpuset")
		}

		return &hostConfig
	}

	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Memory:          job.GetenvInt64("Memory"),
		MemorySwap:      job.GetenvInt64("MemorySwap"),
		CpuShares:       job.GetenvInt64("CpuShares"),
		CpusetCpus:      job.Getenv("CpusetCpus"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
		NetworkMode:     NetworkMode(job.Getenv("NetworkMode")),
		IpcMode:         IpcMode(job.Getenv("IpcMode")),
		PidMode:         PidMode(job.Getenv("PidMode")),
		ReadonlyRootfs:  job.GetenvBool("ReadonlyRootfs"),
		CgroupParent:    job.Getenv("CgroupParent"),
	}

	// FIXME: This is for backward compatibility, if people use `Cpuset`
	// in json, make it workable, we will only pass hostConfig.CpusetCpus
	// to execDriver.
	if job.EnvExists("Cpuset") && hostConfig.CpusetCpus == "" {
		hostConfig.CpusetCpus = job.Getenv("Cpuset")
	}

	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
	job.GetenvJson("Devices", &hostConfig.Devices)
	job.GetenvJson("RestartPolicy", &hostConfig.RestartPolicy)
	job.GetenvJson("Ulimits", &hostConfig.Ulimits)
	job.GetenvJson("LogConfig", &hostConfig.LogConfig)
	hostConfig.SecurityOpt = job.GetenvList("SecurityOpt")
	if Binds := job.GetenvList("Binds"); Binds != nil {
		hostConfig.Binds = Binds
	}
	if Links := job.GetenvList("Links"); Links != nil {
		hostConfig.Links = Links
	}
	if Dns := job.GetenvList("Dns"); Dns != nil {
		hostConfig.Dns = Dns
	}
	if DnsSearch := job.GetenvList("DnsSearch"); DnsSearch != nil {
		hostConfig.DnsSearch = DnsSearch
	}
	if ExtraHosts := job.GetenvList("ExtraHosts"); ExtraHosts != nil {
		hostConfig.ExtraHosts = ExtraHosts
	}
	if VolumesFrom := job.GetenvList("VolumesFrom"); VolumesFrom != nil {
		hostConfig.VolumesFrom = VolumesFrom
	}
	if CapAdd := job.GetenvList("CapAdd"); CapAdd != nil {
		hostConfig.CapAdd = CapAdd
	}
	if CapDrop := job.GetenvList("CapDrop"); CapDrop != nil {
		hostConfig.CapDrop = CapDrop
	}

	return hostConfig
}
