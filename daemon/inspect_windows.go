package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
)

// This sets platform-specific fields
func setPlatformSpecificContainerFields(container *container.Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func inspectExecProcessConfig(e *container.ExecConfig) *backend.ExecProcessConfig {
	return &backend.ExecProcessConfig{
		Tty:        e.Tty,
		Entrypoint: e.Entrypoint,
		Arguments:  e.Args,
	}
}
