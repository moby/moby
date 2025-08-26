package daemon

import (
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	containerpkg "github.com/docker/docker/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(ctr *containerpkg.Container, resp *container.ContainerJSONBase) *container.ContainerJSONBase {
	resp.AppArmorProfile = ctr.AppArmorProfile
	resp.ResolvConfPath = ctr.ResolvConfPath
	resp.HostnamePath = ctr.HostnamePath
	resp.HostsPath = ctr.HostsPath

	return resp
}

func inspectExecProcessConfig(e *containerpkg.ExecConfig) *backend.ExecProcessConfig {
	return &backend.ExecProcessConfig{
		Tty:        e.Tty,
		Entrypoint: e.Entrypoint,
		Arguments:  e.Args,
		Privileged: &e.Privileged,
		User:       e.User,
	}
}
