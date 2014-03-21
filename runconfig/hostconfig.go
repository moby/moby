package runconfig

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/utils"
)

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	LxcConf         []utils.KeyValuePair
	Privileged      bool
	PortBindings    nat.PortMap
	Links           []string
	PublishAllPorts bool
	DriverOptions   map[string][]string
	CliAddress      string
}

type HostConfigForeground struct {
	CliAddressOnly string
}

func ContainerHostConfigFromJob(job *engine.Job, oldHostConfig *HostConfig) *HostConfig {
	if job.EnvExists("CliAddressOnly") {
		hostConfig := HostConfig{}
		if oldHostConfig != nil {
			hostConfig = *oldHostConfig
		}
		hostConfig.CliAddress = job.Getenv("CliAddressOnly")
		return &hostConfig
	}

	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
		CliAddress:      job.Getenv("CliAddress"),
	}
	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
	job.GetenvJson("DriverOptions", &hostConfig.DriverOptions)
	if Binds := job.GetenvList("Binds"); Binds != nil {
		hostConfig.Binds = Binds
	}
	if Links := job.GetenvList("Links"); Links != nil {
		hostConfig.Links = Links
	}

	return hostConfig
}
