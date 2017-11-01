package daemon

import (
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/pkg/sysinfo"
)

// FillPlatformInfo fills the platform related info.
func (daemon *Daemon) FillPlatformInfo(v *types.Info, sysInfo *sysinfo.SysInfo) {
}
