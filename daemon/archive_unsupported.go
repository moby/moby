//go:build !linux && !windows

package daemon

import (
	"io"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
)

func (daemon *Daemon) containerStatPath(*container.Container, string) (*containertypes.PathStat, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "Daemon.containerStatPath"}
}

func (daemon *Daemon) containerArchivePath(*container.Container, string) (io.ReadCloser, *containertypes.PathStat, error) {
	return nil, nil, errdefs.PlatformNotImplemented{Feature: "Daemon.containerArchivePath"}
}

func (daemon *Daemon) containerExtractToDir(*container.Container, string, bool, bool, io.Reader) error {
	return errdefs.PlatformNotImplemented{Feature: "Daemon.containerExtractToDir"}
}
