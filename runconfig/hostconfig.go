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
}

func ContainerHostConfigFromJob(job *engine.Job) *HostConfig {
	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
	}
	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
	if Binds := job.GetenvList("Binds"); Binds != nil {
		hostConfig.Binds = Binds
	}
	if Links := job.GetenvList("Links"); Links != nil {
		hostConfig.Links = Links
	}

	return hostConfig
}
