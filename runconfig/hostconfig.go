package runconfig

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/nat"
)

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	LxcConf         []KeyValuePair
	Privileged      bool
	PortBindings    nat.PortMap
	Links           []string
	PublishAllPorts bool
	CreateOnly      bool
}

type KeyValuePair struct {
	Key   string
	Value string
}

func ContainerHostConfigFromJob(job *engine.Job) *HostConfig {
	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
		CreateOnly:      job.GetenvBool("CreateOnly"),
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
