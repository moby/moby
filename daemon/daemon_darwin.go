//go:build !linux && !freebsd && !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/remote"
	"github.com/docker/docker/libnetwork"
	nwconfig "github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/sysinfo"
)

const (
	isWindows        = false
	cgroupNoneDriver = "none"
)

func checkSystem() error {
	return nil
}

func setupResolvConf(config *config.Config) {
}

func getSysInfo(cfg *config.Config) *sysinfo.SysInfo {
	return sysinfo.New()
}

func cgroupDriver(cfg *config.Config) string {
	return cgroupNoneDriver
}

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, daemonCfg *config.Config, ec *container.ExecConfig, p *specs.Process) error {
	return nil
}

func (daemon *Daemon) parseSecurityOpt(daemonCfg *config.Config, securityOptions *container.SecurityOptions, hostConfig *containertypes.HostConfig) error {
	return nil
}

func (daemon *Daemon) registerLinks(container *container.Container, hostConfig *containertypes.HostConfig) error {
	return nil
}

func verifyPlatformContainerSettings(daemon *Daemon, daemonCfg *configStore, hostConfig *containertypes.HostConfig, update bool) (warnings []string, err error) {
	return nil, nil
}

func (daemon *Daemon) adaptContainerSettings(daemonCfg *config.Config, hostConfig *containertypes.HostConfig, adjustCPUShares bool) error {
	return nil
}

func adjustParallelLimit(n int, limit int) int {
	return limit
}

func setupInitLayer(idMapping idtools.IdentityMapping) func(string) error {
	return nil
}

func (daemon *Daemon) initNetworkController(cfg *config.Config, activeSandboxes map[string]interface{}) error {
	netOptions, err := daemon.networkOptions(cfg, daemon.PluginStore, activeSandboxes)
	if err != nil {
		return err
	}

	daemon.netController, err = libnetwork.New(netOptions...)
	if err != nil {
		return fmt.Errorf("error obtaining controller instance: %v", err)
	}

	if len(activeSandboxes) > 0 {
		log.G(context.TODO()).Info("there are running containers, updated network configuration will not take affect")
	} else if err := configureNetworking(daemon.netController, cfg); err != nil {
		return err
	}

	return nil
}

func configureNetworking(controller *libnetwork.Controller, conf *config.Config) error {
	// Initialize default network on "null"
	if n, _ := controller.NetworkByName("none"); n == nil {
		if _, err := controller.NewNetwork("null", "none", "", libnetwork.NetworkOptionPersist(true)); err != nil {
			return errors.Wrap(err, `error creating default "null" network`)
		}
	}

	// Initialize default network on "host"
	if n, _ := controller.NetworkByName("host"); n == nil {
		if _, err := controller.NewNetwork("host", "host", "", libnetwork.NetworkOptionPersist(true)); err != nil {
			return errors.Wrap(err, `error creating default "host" network`)
		}
	}

	return nil
}

func verifyDaemonSettings(config *config.Config) error {
	return nil
}

func setupRemappedRoot(config *config.Config) (idtools.IdentityMapping, error) {
	return idtools.IdentityMapping{}, nil
}

func (daemon *Daemon) setupSeccompProfile(*config.Config) error {
	return nil
}

func (daemon *Daemon) setDefaultIsolation(*config.Config) error {
	return nil
}

func (daemon *Daemon) cleanupMountsByID(in string) error {
	return nil
}

func (daemon *Daemon) cleanupMounts(*config.Config) error {
	return nil
}

func (daemon *Daemon) initLibcontainerd(ctx context.Context, cfg *config.Config) error {
	var err error
	daemon.containerd, err = remote.NewClient(
		ctx,
		daemon.containerdClient,
		filepath.Join(cfg.ExecRoot, "containerd"),
		cfg.ContainerdNamespace,
		daemon,
	)
	return err
}

func setMayDetachMounts() error {
	return nil
}

func getPluginExecRoot(config *config.Config) string {
	return filepath.Join(config.ExecRoot, "plugins")
}

func configureMaxThreads(config *config.Config) error {
	return nil
}

func configureKernelSecuritySupport(config *config.Config, driverName string) error {
	return nil
}

func setupDaemonRoot(config *config.Config, rootDir string, rootIdentity idtools.Identity) error {
	config.Root = rootDir
	return os.MkdirAll(rootDir, 0o755)
}

func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	return daemon.Mount(container)
}

func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) error {
	return daemon.Unmount(container)
}

func driverOptions(_ *config.Config) nwconfig.Option {
	return nil
}

func UsingSystemd(config *config.Config) bool {
	return false
}

func recursiveUnmount(target string) error {
	return mount.UnmountRecursive(target, unix.MNT_FORCE)
}
