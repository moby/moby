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

	// Buildkit breaks when userns remapping is enabled and containerd snapshotter is used. As a temporary workaround,
	// if containerd snapshotter is explicitly enabled, and userns remapping is enabled too, return an error. If userns
	// remapping is enabled, but containerd-snapshotter is enabled by default, disable it. See https://github.com/moby/moby/issues/47377.
	enabled := conf.Features["containerd-snapshotter"]
	if enabled {
		return errors.New("containerd-snapshotter is explicitly enabled, but is not compatible with userns remapping. Please disable userns remapping or containerd-snapshotter")
	}

	log.G(context.TODO()).Warn("userns remapping enabled, disabling containerd snapshotter")
	if conf.Features == nil {
		conf.Features = make(map[string]bool)
	}
	conf.Features["containerd-snapshotter"] = false

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
