//go:build !linux && !windows

package daemon

import (
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
)

func (daemon *Daemon) stats(*container.Container) (*containertypes.StatsResponse, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "Daemon.stats"}
}

func (daemon *Daemon) getNetworkStats(*container.Container) (map[string]containertypes.NetworkStats, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "Daemon.getNetworkStats"}
}

func getSystemCPUUsage() (uint64, uint32, error) {
	return 0, 0, errdefs.PlatformNotImplemented{Feature: "getSystemCPUUsage"}
}
