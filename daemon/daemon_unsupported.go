//go:build !linux && !freebsd && !windows

package daemon

import (
	"errors"

	"github.com/moby/moby/v2/pkg/sysinfo"
)

func checkSystem() error {
	return errors.New("the Docker daemon is not supported on this platform")
}

func setupResolvConf(_ *any) {}

func getSysInfo(_ *Daemon) *sysinfo.SysInfo {
	return sysinfo.New()
}

func createCGroup2Root(_ context.Context, _ *config.Config) {}
