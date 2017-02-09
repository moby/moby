package container

import (
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/swarmkit/api"
)

func applySecurityConfig(cfg *enginecontainer.HostConfig, sec *api.ContainerSpec_SecurityConfig) {
	if sec == nil {
		return
	}
	m := enginecontainer.UsernsMode(sec.Userns)
	cfg.UsernsMode = m
}
