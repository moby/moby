package command

import (
	"context"

	cdcgroups "github.com/containerd/cgroups/v3"
	"github.com/containerd/log"
	systemdDaemon "github.com/coreos/go-systemd/v22/daemon"
	"github.com/moby/moby/v2/daemon"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/pkg/sysinfo"
	"github.com/pkg/errors"
)

// setPlatformOptions applies platform-specific CLI configuration options.
func setPlatformOptions(conf *config.Config) error {
	if conf.RemappedRoot == "" {
		return nil
	}

	containerdNamespace, containerdPluginNamespace, err := daemon.RemapContainerdNamespaces(conf)
	if err != nil {
		return err
	}
	conf.ContainerdNamespace = containerdNamespace
	conf.ContainerdPluginNamespace = containerdPluginNamespace

	return nil
}

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() error {
	return nil
}

// notifyReady sends a message to the host when the server is ready to be used
func notifyReady() {
	// Tell the init daemon we are accepting requests
	go systemdDaemon.SdNotify(false, systemdDaemon.SdNotifyReady)
}

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
	go systemdDaemon.SdNotify(false, systemdDaemon.SdNotifyStopping)
}

func validateCPURealtimeOptions(cfg *config.Config) error {
	if cfg.CPURealtimePeriod == 0 && cfg.CPURealtimeRuntime == 0 {
		return nil
	}
	if cdcgroups.Mode() == cdcgroups.Unified {
		return errors.New("daemon-scoped cpu-rt-period and cpu-rt-runtime are not implemented for cgroup v2")
	}
	if !sysinfo.New().CPURealtime {
		return errors.New("daemon-scoped cpu-rt-period and cpu-rt-runtime are not supported by the kernel")
	}
	return nil
}

// Buildkit breaks when userns remapping is enabled, and containerd snapshotter is used. As a workaround, disable
// containerd snapshotter if userns remapping is enabled. See https://github.com/moby/moby/issues/47377.
func disableC8dSnapshotterOnUsernsRemap(cfg *config.Config) {
	if cfg.RemappedRoot != "" {
		if enabled, ok := cfg.Features["containerd-snapshotter"]; !ok || enabled {
			log.G(context.TODO()).Warn("userns remapping enabled, disabling containerd snapshotter")
		}
		if cfg.Features == nil {
			cfg.Features = make(map[string]bool)
		}
		cfg.Features["containerd-snapshotter"] = false
	}
}
