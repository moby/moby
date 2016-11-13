package daemon

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/sysinfo"
)

func (daemon *Daemon) FillPlatformInfo(v *types.InfoBase, sysInfo *sysinfo.SysInfo) {
}
