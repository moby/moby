package command

import "github.com/docker/docker/daemon/config"

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() error {
	return nil
}

// notifyReady sends a message to the host when the server is ready to be used
func notifyReady() {
}

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
}

func validateCPURealtimeOptions(_ *config.Config) error {
	return nil
}
