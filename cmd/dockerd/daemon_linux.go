package main

import (
	cdcgroups "github.com/containerd/cgroups/v3"
	systemdDaemon "github.com/coreos/go-systemd/v22/daemon"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/sysinfo"
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
	_, _ = systemdDaemon.SdNotify(false, systemdDaemon.SdNotifyReady)
}

// notifyReloading sends a message to the host when the server got signaled to
// reloading its configuration, see [sd_notify(3)]. The server should be running
// as a systemd unit with "Type=notify" (see [systemd.service(5)]).
//
// notifyReloading returns a callback that must be called after reloading completes
// (either successfully or unsuccessfully) to send [notifyReady]
//
// Note: we currently use the pre-systemd 253 implementation, which uses "Type=notify",
// combined with [ExecReload]. The [ExecReload] option is designed to be synchronous,
// which complicates its use when signals (SIGHUP) is used to reload:
//
// > Note however that reloading a daemon by enqueuing a signal (...) is usually
// > not a good choice, because this is an asynchronous operation and hence not
// > suitable when ordering reloads of multiple services against each other.
//
// Systemd 253 introduced a new Type (Type=notify-reload, see [systemd#25916]),
// which allows setting a "ReloadSignal" instead, and requires sending a
// [MONOTONIC_USEC] message.
//
// We currently still support distros that do not provide systemd 253, so cannot
// use this new feature, but sending "RELOADING=1 / "READY=1" should at least
// provide more information to systemd for the time being.
//
// [sd_notify(3)]: https://www.freedesktop.org/software/systemd/man/latest/sd_notify.html#RELOADING=1
// [systemd.service(5)]: https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html#Type=
// [ExecReload]: https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html#ExecReload=
// [MONOTONIC_USEC]: https://www.freedesktop.org/software/systemd/man/latest/sd_notify.html#MONOTONIC_USEC=…
// [systemd#25916]: https://github.com/systemd/systemd/pull/25916
func notifyReloading() (done func()) {
	// TODO(thaJeztah): Set "MONOTONIC_USEC" once we drop support for systemd < 253, and supported by github.com/coreos/go-systemd, and update systemd unit accordingly; see https://github.com/moby/moby/pull/47358#discussion_r1483533255.
	sent, _ := systemdDaemon.SdNotify(false, systemdDaemon.SdNotifyReloading)
	if !sent {
		// Nothing to do if no reloading event was sent.
		return func() {}
	}
	return notifyReady
}

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
	_, _ = systemdDaemon.SdNotify(false, systemdDaemon.SdNotifyStopping)
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
