package api

import (
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/runconfig"
)

type Container struct {
	Config     runconfig.Config
	HostConfig runconfig.HostConfig
	State      struct {
		Running  bool
		ExitCode int
	}
	NetworkSettings struct {
		Ports nat.PortMap
	}
}
