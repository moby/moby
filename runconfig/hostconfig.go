package runconfig

import (
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/utils"
)

type NetworkMode string

func (n NetworkMode) IsHost() bool {
	return n == "host"
}

func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
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
	VolumesFrom     []string
	Devices         []DeviceMapping
	NetworkMode     NetworkMode
	CapAdd          []string
	CapDrop         []string
	RestartPolicy   RestartPolicy
}

func ContainerHostConfigFromJob(job *engine.Job) *HostConfig {
	hostConfig := &HostConfig{
		ContainerIDFile: job.Getenv("ContainerIDFile"),
		Privileged:      job.GetenvBool("Privileged"),
		PublishAllPorts: job.GetenvBool("PublishAllPorts"),
		NetworkMode:     NetworkMode(job.Getenv("NetworkMode")),
	}

	job.GetenvJson("LxcConf", &hostConfig.LxcConf)
	job.GetenvJson("PortBindings", &hostConfig.PortBindings)
	job.GetenvJson("Devices", &hostConfig.Devices)
	job.GetenvJson("RestartPolicy", &hostConfig.RestartPolicy)
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
	if CapAdd := job.GetenvList("CapAdd"); CapAdd != nil {
		hostConfig.CapAdd = CapAdd
	}
	if CapDrop := job.GetenvList("CapDrop"); CapDrop != nil {
		hostConfig.CapDrop = CapDrop
	}

	return hostConfig
}
