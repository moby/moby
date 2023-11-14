package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containerd/log"
	"golang.org/x/sys/windows"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/system"
)

func getDefaultDaemonConfigFile() (string, error) {
	return "", nil
}

// setDefaultUmask doesn't do anything on windows
func setDefaultUmask() error {
	return nil
}

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() {
	// start the service now to prevent timeouts waiting for daemon to start
	// but still (eventually) complete all requests that are sent after this
	if service != nil {
		err := service.started()
		if err != nil {
			log.G(context.TODO()).Fatal(err)
		}
	}
}

// notifyReady sends a message to the host when the server is ready to be used
func notifyReady() {
}

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
}

// notifyShutdown is called after the daemon shuts down but before the process exits.
func notifyShutdown(err error) {
	if service != nil {
		if err != nil {
			log.G(context.TODO()).Fatal(err)
		}
		service.stopped(err)
	}
}

// setupConfigReloadTrap configures a Win32 event to reload the configuration.
func (cli *DaemonCli) setupConfigReloadTrap() {
	go func() {
		sa := windows.SecurityAttributes{
			Length: 0,
		}
		event := "Global\\docker-daemon-config-" + fmt.Sprint(os.Getpid())
		ev, _ := windows.UTF16PtrFromString(event)
		if h, _ := windows.CreateEvent(&sa, 0, 0, ev); h != 0 {
			log.G(context.TODO()).Debugf("Config reload - waiting signal at %s", event)
			for {
				windows.WaitForSingleObject(h, windows.INFINITE)
				cli.reloadConfig()
			}
		}
	}()
}

// getSwarmRunRoot gets the root directory for swarm to store runtime state
// For example, the control socket
func (cli *DaemonCli) getSwarmRunRoot() string {
	return ""
}

func allocateDaemonPort(addr string) error {
	return nil
}

func newCgroupParent(config *config.Config) string {
	return ""
}

func (cli *DaemonCli) initContainerd(_ context.Context) (func(time.Duration) error, error) {
	system.InitContainerdRuntime(cli.ContainerdAddr)
	return nil, nil
}

func validateCPURealtimeOptions(_ *config.Config) error {
	return nil
}
