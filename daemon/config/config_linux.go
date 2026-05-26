package config

import (
	"context"
	"path/filepath"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/log"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/pkg/opts"
	"github.com/moby/moby/v2/pkg/homedir"
	"github.com/pkg/errors"
)

const (
	// DefaultCgroupV1NamespaceMode is the default mode for containers cgroup namespace when using cgroups v1.
	DefaultCgroupV1NamespaceMode = container.CgroupnsModeHost

	// userlandProxyBinary is the name of the userland-proxy binary.
	userlandProxyBinary = "docker-proxy"
)

func setPlatformDefaults(cfg *Config) error {
	cfg.Ulimits = make(map[string]*container.Ulimit)
	cfg.ShmSize = opts.MemBytes(DefaultShmSize)
	cfg.SeccompProfile = SeccompProfileDefault
	cfg.IpcMode = string(DefaultIpcMode)
	cfg.Runtimes = make(map[string]system.Runtime)

	if cgroups.Mode() != cgroups.Unified {
		cfg.CgroupNamespaceMode = string(DefaultCgroupV1NamespaceMode)
	} else {
		cfg.CgroupNamespaceMode = string(DefaultCgroupNamespaceMode)
	}

	var err error
	cfg.BridgeConfig.EnableUserlandProxy = true
	cfg.BridgeConfig.UserlandProxyPath, err = lookupBinPath(userlandProxyBinary)
	if err != nil {
		// Log, but don't error here. This allows running a daemon with
		// userland-proxy disabled (which does not require the binary
		// to be present).
		//
		// An error is still produced by [Config.ValidatePlatformConfig] if
		// userland-proxy is enabled in the configuration.
		//
		// We log this at "debug" level, as this code is also executed
		// when running "--version", and we don't want to print logs in
		// that case..
		log.G(context.TODO()).WithError(err).Debug("failed to lookup default userland-proxy binary")
	}

	if rootless.RunningWithRootlessKit() {
		cfg.Rootless = true

		dataHome, err := homedir.GetDataHome()
		if err != nil {
			return err
		}
		runtimeDir, err := homedir.GetRuntimeDir()
		if err != nil {
			return err
		}

		cfg.Root = filepath.Join(dataHome, "docker")
		cfg.ExecRoot = filepath.Join(runtimeDir, "docker")
		cfg.Pidfile = filepath.Join(runtimeDir, "docker.pid")
	} else {
		cfg.Root = "/var/lib/docker"
		cfg.ExecRoot = "/var/run/docker"
		cfg.Pidfile = "/var/run/docker.pid"
	}

	return nil
}

// validatePlatformConfig checks if any platform-specific configuration settings are invalid.
func validatePlatformConfig(conf *Config) error {
	if err := verifyUserlandProxyConfig(conf); err != nil {
		return err
	}
	if err := verifyDefaultIpcMode(conf.IpcMode); err != nil {
		return err
	}
	if err := bridge.ValidateFixedCIDRV6(conf.FixedCIDRv6); err != nil {
		return errors.Wrap(err, "invalid fixed-cidr-v6")
	}
	if err := validateFirewallBackend(conf.FirewallBackend); err != nil {
		return errors.Wrap(err, "invalid firewall-backend")
	}
	if err := validateFwMarkMask(conf.BridgeAcceptFwMark); err != nil {
		return errors.Wrap(err, "invalid bridge-accept-fwmark")
	}
	return verifyDefaultCgroupNsMode(conf.CgroupNamespaceMode)
}
