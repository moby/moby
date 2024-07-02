package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	containerpkg "github.com/docker/docker/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *containerpkg.Container, contJSONBase *container.InspectBase) *container.InspectBase {
	contJSONBase.AppArmorProfile = container.AppArmorProfile
	contJSONBase.ResolvConfPath = container.ResolvConfPath
	contJSONBase.HostnamePath = container.HostnamePath
	contJSONBase.HostsPath = container.HostsPath

	return contJSONBase
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
