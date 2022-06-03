//go:build !linux && !freebsd && !windows
// +build !linux,!freebsd,!windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/sysinfo"
)

const platformSupported = false

func setupResolvConf(config *config.Config) {
}

func getSysInfo(daemon *Daemon) *sysinfo.SysInfo {
	return sysinfo.New()
}
