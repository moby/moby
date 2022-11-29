package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/supervisor"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

func getDefaultDaemonConfigFile() (string, error) {
	return "", nil
}

// setDefaultUmask doesn't do anything on windows
func setDefaultUmask() error {
	return nil
}

func getDaemonConfDir(root string) (string, error) {
	return filepath.Join(root, "config"), nil
}

// preNotifyReady sends a message to the host when the API is active, but before the daemon is
func preNotifyReady() {
	// start the service now to prevent timeouts waiting for daemon to start
	// but still (eventually) complete all requests that are sent after this
	if service != nil {
		err := service.started()
		if err != nil {
			logrus.Fatal(err)
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
			logrus.Fatal(err)
		}
		service.stopped(err)
	}
}

func (cli *DaemonCli) getPlatformContainerdDaemonOpts() ([]supervisor.DaemonOpt, error) {
	opts := []supervisor.DaemonOpt{
		// On Windows, it first checks if a containerd binary is found in the same
		// directory as the dockerd binary. If found, this binary takes precedence
		// over containerd binaries installed in $PATH.
		supervisor.WithDetectLocalBinary(),
	}

	return opts, nil
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
			logrus.Debugf("Config reload - waiting signal at %s", event)
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

func (cli *DaemonCli) initContainerd(ctx context.Context) (func(time.Duration) error, error) {
	if cli.ContainerdAddr != "" {
		// use system containerd at the given address.
		system.InitContainerdRuntime(cli.ContainerdAddr)
		return nil, nil
	}

	if cli.DefaultRuntime != config.WindowsV2RuntimeName {
		return nil, nil
	}

	logrus.Info("containerd not running, starting managed containerd")
	opts, err := cli.getContainerdDaemonOpts()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate containerd options")
	}

	r, err := supervisor.Start(ctx, filepath.Join(cli.Root, "containerd"), filepath.Join(cli.ExecRoot, "containerd"), opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start containerd")
	}
	cli.ContainerdAddr = r.Address()
	system.InitContainerdRuntime(r.Address())

	// Try to wait for containerd to shutdown
	return r.WaitTimeout, nil
}

func validateCPURealtimeOptions(_ *config.Config) error {
	return nil
}
