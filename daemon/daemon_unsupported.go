//go:build !linux && !freebsd && !windows

package daemon

import (
	"context"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	nwconfig "github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/moby/v2/pkg/sysinfo"
	"github.com/moby/sys/user"
)

const (
	isWindows = false

	// Constants from the Linux daemon required by shared code paths.
	cgroupFsDriver      = "cgroupfs"
	cgroupSystemdDriver = "systemd"
	cgroupNoneDriver    = "none"
)

func cgroupDriver(*config.Config) string {
	return cgroupNoneDriver
}

func checkSystem() error {
	return errdefs.PlatformNotImplemented{Feature: "the Docker daemon"}
}

func setupResolvConf(*config.Config) {}

func getSysInfo(*config.Config) *sysinfo.SysInfo {
	return sysinfo.New()
}

func (daemon *Daemon) runInNetNS(f func() error) error {
	return f()
}

func (daemon *Daemon) parseSecurityOpt(_ *config.Config, securityOptions *container.SecurityOptions, hostConfig *containertypes.HostConfig) error {
	return parseSecurityOpt(securityOptions, hostConfig)
}

func parseSecurityOpt(*container.SecurityOptions, *containertypes.HostConfig) error {
	return nil
}

func verifyPlatformContainerSettings(_ *Daemon, _ *configStore, _ *containertypes.HostConfig, _ bool) ([]string, error) {
	return nil, nil
}

func (daemon *Daemon) adaptContainerSettings(*config.Config, *containertypes.HostConfig) error {
	return nil
}

func (daemon *Daemon) registerLinks(*container.Container) error {
	return nil
}

func setupInitLayer(_, _ int) func(string) error {
	return func(string) error { return nil }
}

func adjustParallelLimit(_ int, limit int) int {
	return limit
}

func verifyDaemonSettings(*config.Config) error {
	return nil
}

func setupRemappedRoot(*config.Config) (user.IdentityMapping, error) {
	return user.IdentityMapping{}, nil
}

func (daemon *Daemon) setupSeccompProfile(*config.Config) error {
	return nil
}

func (daemon *Daemon) setDefaultIsolation(*config.Config) error {
	return nil
}

func configureMaxThreads(context.Context) error {
	return nil
}

func getPluginExecRoot(cfg *config.Config) string {
	return cfg.ExecRoot
}

func (daemon *Daemon) initNetworkController(*config.Config, map[string]any) error {
	return nil
}

// networkPlatformOptions returns a slice of platform-specific libnetwork options.
func networkPlatformOptions(*config.Config) []nwconfig.Option {
	return nil
}

func configureKernelSecuritySupport(*config.Config, string) error {
	return nil
}

func (daemon *Daemon) cleanupMounts(*config.Config) error { return nil }

func (daemon *Daemon) cleanupMountsByID(string) error { return nil }

func setupDaemonRoot(*config.Config, string, int, int) error {
	return nil
}

// UsingSystemd returns true if cli option includes native.cgroupdriver=systemd.
// systemd is Linux-only, so this is always false on unsupported platforms.
func UsingSystemd(*config.Config) bool {
	return false
}

func (daemon *Daemon) conditionalMountOnStart(*container.Container) error {
	return nil
}

func (daemon *Daemon) conditionalUnmountOnCleanup(*container.Container) error {
	return nil
}

func recursiveUnmount(string) error {
	return nil
}

func (daemon *Daemon) setupContainerDirs(*container.Container) ([]container.Mount, error) {
	return nil, nil
}
