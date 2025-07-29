package daemon

import (
	"github.com/moby/moby/api/types/container"

	containerpkg "github.com/docker/docker/daemon/container"
	"github.com/docker/docker/daemon/server/backend"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *containerpkg.Container, contJSONBase *container.ContainerJSONBase) *container.ContainerJSONBase {
	return contJSONBase
}

func inspectExecProcessConfig(e *containerpkg.ExecConfig) *backend.ExecProcessConfig {
	return &backend.ExecProcessConfig{
		Tty:        e.Tty,
		Entrypoint: e.Entrypoint,
		Arguments:  e.Args,
	}
}
