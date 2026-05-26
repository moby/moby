//go:build !linux && !windows

package daemon

import (
	"context"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
)

// containerFSView is a placeholder for platforms without a real
// container filesystem implementation. Methods return
// [errdefs.PlatformNotImplemented] so callers in shared Unix code paths
// fail at runtime rather than at compile time.
type containerFSView struct{}

func (daemon *Daemon) openContainerFS(*container.Container) (*containerFSView, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "openContainerFS"}
}

func (vw *containerFSView) Close() error { return nil }

func (vw *containerFSView) Stat(context.Context, string) (*containertypes.PathStat, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "containerFSView.Stat"}
}

func (vw *containerFSView) RunInFS(context.Context, func() error) error {
	return errdefs.PlatformNotImplemented{Feature: "containerFSView.RunInFS"}
}

func (vw *containerFSView) GoInFS(context.Context, func()) error {
	return errdefs.PlatformNotImplemented{Feature: "containerFSView.GoInFS"}
}
