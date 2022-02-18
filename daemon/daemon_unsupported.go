//go:build !linux && !freebsd && !windows
// +build !linux,!freebsd,!windows

package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/daemon/config"
	"github.com/moby/moby/pkg/sysinfo"
)

const platformSupported = false

func setupResolvConf(config *config.Config) {
}

func (daemon *Daemon) loadSysInfo() {
	daemon.sysInfo = sysinfo.New()
}
