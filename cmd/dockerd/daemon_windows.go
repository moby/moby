package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/system"
	"golang.org/x/sys/windows"
)

// getDefaultDaemonConfigFile returns the default location of the daemon's
// configuration file.
//
// On Windows, the location of the config-file is relative to the daemon's
// data-root (config.Root), which is configurable, so we cannot use a fixed
// default location, and this function always returns an empty string.
func getDefaultDaemonConfigFile() string {
	return ""
}

// setPlatformOptions applies platform-specific CLI configuration options.
func setPlatformOptions(cfg *config.Config) error {
	if cfg.Pidfile == "" {
		// On Windows, the pid-file location is relative to the daemon's data-root,
		// which is configurable, so we cannot use a fixed default location.
		// Instead, we set the location here, after we parsed command-line flags
		// and loaded the configuration file (if any).
		cfg.Pidfile = filepath.Join(cfg.Root, "docker.pid")
	}
	return nil
}

// setDefaultUmask doesn't do anything on windows
func setDefaultUmask() error {
	return nil
}

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() error {
	// start the service now to prevent timeouts waiting for daemon to start
	// but still (eventually) complete all requests that are sent after this
	if service != nil {
		err := service.started()
		if err != nil {
			return err
		}
	}
	return nil
}

// notifyReady sends a message to the host when the server is ready to be used
func notifyReady() {
}

// notifyReloading sends a message to the host when the server got signaled to
// reloading its configuration. It is a no-op on Windows.
func notifyReloading() func() { return func() {} }

// notifyStopping sends a message to the host when the server is shutting down
func notifyStopping() {
}

// notifyShutdown is called after the daemon shuts down but before the process exits.
func notifyShutdown(err error) {
	if service != nil {
		service.stopped(err)
	}
}

// setupConfigReloadTrap configures a Win32 event to reload the configuration.
func (cli *daemonCLI) setupConfigReloadTrap() {
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
func getSwarmRunRoot(*config.Config) string {
	return ""
}

func allocateDaemonPort(addr string) error {
	return nil
}

func newCgroupParent(*config.Config) string {
	return ""
}

func (cli *daemonCLI) initContainerd(ctx context.Context) (func(time.Duration) error, error) {
	defer func() { system.EnableContainerdRuntime(cli.Config.ContainerdAddr) }()

	if cli.Config.ContainerdAddr != "" {
		return nil, nil
	}

	if cli.Config.DefaultRuntime != config.WindowsV2RuntimeName {
		return nil, nil
	}

	return cli.initializeContainerd(ctx)
}

func validateCPURealtimeOptions(_ *config.Config) error {
	return nil
}
