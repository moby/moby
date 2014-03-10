package runconfig

import (
	"strings"

	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/utils"
)

type NetworkMode string

func (n NetworkMode) IsHost() bool {
	return n == "host"
}

func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
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
	VolumesFrom     []string
	NetworkMode     NetworkMode
}

// This is used by the create command when you want to both set the
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
	}
	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
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
	if VolumesFrom := job.GetenvList("VolumesFrom"); VolumesFrom != nil {
		hostConfig.VolumesFrom = VolumesFrom
	}
	return hostConfig
}
