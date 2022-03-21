package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/sysinfo"
)

// fillPlatformInfo fills the platform related info.
func (daemon *Daemon) fillPlatformInfo(_ context.Context, v *types.Info, sysInfo *sysinfo.SysInfo) {
}

func (daemon *Daemon) fillPlatformVersion(_ context.Context, v *types.Version) {}

func fillDriverWarnings(v *types.Info) {
}

func (daemon *Daemon) cgroupNamespacesEnabled(sysInfo *sysinfo.SysInfo) bool {
	return false
}

// Rootless returns true if daemon is running in rootless mode
func (daemon *Daemon) Rootless() bool {
	return false
}
