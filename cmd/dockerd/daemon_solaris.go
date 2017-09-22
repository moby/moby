// +build solaris

package main

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/docker/docker/libcontainerd"
	"golang.org/x/sys/unix"
)

const defaultDaemonConfigFile = ""

// setDefaultUmask sets the umask to 0022 to avoid problems
// caused by custom umask
func setDefaultUmask() error {
	desiredUmask := 0022
	unix.Umask(desiredUmask)
	if umask := unix.Umask(desiredUmask); umask != desiredUmask {
		return fmt.Errorf("failed to set umask: expected %#o, got %#o", desiredUmask, umask)
	}

	return nil
}

func getDaemonConfDir(_ string) string {
	return "/etc/docker"
}

// setupConfigReloadTrap configures the USR2 signal to reload the configuration.
func (cli *DaemonCli) setupConfigReloadTrap() {
}

// preNotifySystem sends a message to the host when the API is active, but before the daemon is
func preNotifySystem() {
}

// notifySystem sends a message to the host when the server is ready to be used
func notifySystem() {
}

func (cli *DaemonCli) getPlatformRemoteOptions() ([]libcontainerd.RemoteOption, error) {
	return nil, nil
}

// getSwarmRunRoot gets the root directory for swarm to store runtime state
// For example, the control socket
func (cli *DaemonCli) getSwarmRunRoot() string {
	return filepath.Join(cli.Config.ExecRoot, "swarm")
}

func allocateDaemonPort(addr string) error {
	return nil
}

// notifyShutdown is called after the daemon shuts down but before the process exits.
func notifyShutdown(err error) {
}

func wrapListeners(proto string, ls []net.Listener) []net.Listener {
	return ls
}
