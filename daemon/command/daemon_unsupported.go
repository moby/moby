//go:build !linux && !windows

package command

import "github.com/moby/moby/v2/daemon/config"

// setPlatformOptions is a no-op on platforms without daemon-side userns remapping.
func setPlatformOptions(*config.Config) error {
	return nil
}

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() error {
	return nil
}

// notifyReady sends a message to the host when the server is ready to be used
func notifyReady() {
}

// notifyReloading sends a message to the host when the server got signaled to
// reloading its configuration. It is a no-op on platforms without systemd.
func notifyReloading() func() { return func() {} }

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
}

func validateCPURealtimeOptions(*config.Config) error {
	return nil
}
