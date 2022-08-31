package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/sysinfo"
)

// fillPlatformInfo fills the platform related info.
func (daemon *Daemon) fillPlatformInfo(v *types.Info, sysInfo *sysinfo.SysInfo, cfg *configStore) {
}

func (daemon *Daemon) fillPlatformVersion(v *types.Version, cfg *configStore) {}

func fillDriverWarnings(v *types.Info) {
}

func cgroupNamespacesEnabled(sysInfo *sysinfo.SysInfo, cfg *config.Config) bool {
	return false
}

// Rootless returns true if daemon is running in rootless mode
func Rootless(*config.Config) bool {
	return false
}

func noNewPrivileges(*config.Config) bool {
	return false
}
