//go:build !linux

package daemon

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
)

type containerFSView struct{}

func (vw *containerFSView) Close() error {
	return nil
}

func (vw *containerFSView) GoInFS(ctx context.Context, fn func()) error {
	return nil
}

func (vw *containerFSView) Stat(ctx context.Context, path string) (*types.ContainerPathStat, error) {
	return nil, nil
}

func (vw *containerFSView) RunInFS(ctx context.Context, fn func() error) error {
	return nil
}

func (daemon *Daemon) openContainerFS(container *container.Container) (_ *containerFSView, err error) {
	return nil, nil
}
