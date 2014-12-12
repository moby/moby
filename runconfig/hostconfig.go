package runconfig

import (
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
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

type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	LxcConf         []utils.KeyValuePair
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
	CapAdd          []string
	CapDrop         []string
	RestartPolicy   RestartPolicy
	SecurityOpt     []string
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
		return &hostConfig
	}

	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
		NetworkMode:     NetworkMode(job.Getenv("NetworkMode")),
		IpcMode:         IpcMode(job.Getenv("IpcMode")),
	}

	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
	job.GetenvJson("Devices", &hostConfig.Devices)
	job.GetenvJson("RestartPolicy", &hostConfig.RestartPolicy)
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
