//go:build !linux && !freebsd && !windows
// +build !linux,!freebsd,!windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"errors"

	"github.com/docker/docker/pkg/sysinfo"
)

func checkSystem() error {
	return errors.New("the Docker daemon is not supported on this platform")
}

func setupResolvConf(_ *interface{}) {}

func getSysInfo(_ *Daemon) *sysinfo.SysInfo {
	return sysinfo.New()
}
