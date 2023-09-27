package daemon

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/container"
)

func setPlatformSpecificContainerFields(container *container.Container, contJSONBase *types.ContainerJSONBase) *types.ContainerJSONBase {
	return contJSONBase
}

func (daemon *Daemon) containerInspectPre120(ctx context.Context, name string) (*types.ContainerJSON, error) {
	return daemon.ContainerInspectCurrent(ctx, name, false)
}

func inspectExecProcessConfig(e *container.ExecConfig) *backend.ExecProcessConfig {
	return &backend.ExecProcessConfig{
		Tty:        e.Tty,
		Entrypoint: e.Entrypoint,
		Arguments:  e.Args,
		Privileged: &e.Privileged,
		User:       e.User,
	}
}
