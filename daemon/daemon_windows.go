package daemon

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
)

func (daemon *Daemon) Changes(container *Container) ([]archive.Change, error) {
	return daemon.driver.Changes(container.ID, container.ImageID)
}

func (daemon *Daemon) Diff(container *Container) (archive.Archive, error) {
	return daemon.driver.Diff(container.ID, container.ImageID)
}

func parseSecurityOpt(container *Container, config *runconfig.HostConfig) error {
	return nil
}

func (daemon *Daemon) createRootfs(container *Container) error {
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return err
	}
	if err := daemon.driver.Create(container.ID, container.ImageID); err != nil {
		return err
	}
	return nil
}

func checkKernel() error {
	return nil
}

func (daemon *Daemon) verifyContainerSettings(hostConfig *runconfig.HostConfig, config *runconfig.Config) ([]string, error) {
	// TODO Windows. Verifications TBC
	return nil, nil
}

// checkConfigOptions checks for mutually incompatible config options
func checkConfigOptions(config *Config) error {
	return nil
}

// checkSystem validates the system is supported and we have sufficient privileges
func checkSystem() error {
	var dwVersion uint32

	// TODO Windows. Once daemon is running on Windows, move this code back to
	// NewDaemon() in daemon.go, and extend the check to support Windows.
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return ErrSystemNotSupported
	}

	// TODO Windows. May need at some point to ensure have elevation and
	// possibly LocalSystem.

	// Validate the OS version. Note that docker.exe must be manifested for this
	// call to return the correct version.
	dwVersion, err := syscall.GetVersion()
	if err != nil {
		return fmt.Errorf("Failed to call GetVersion()")
	}
	if int(dwVersion&0xFF) < 10 {
		return fmt.Errorf("This version of Windows does not support the docker daemon")
	}

	return nil
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *Config, driverName string) error {
	return nil
}

func migrateIfDownlevel(driver graphdriver.Driver, root string) error {
	return nil
}

func configureVolumes(config *Config) error {
	// Windows does not support volumes at this time
	return nil
}

func configureSysInit(config *Config) (string, error) {
	// TODO Windows.
	return os.Getenv("TEMP"), nil
}

func isNetworkDisabled(config *Config) bool {
	return false
}

func initNetworkController(config *Config) (libnetwork.NetworkController, error) {
	// TODO Windows
	return nil, nil
}

func (daemon *Daemon) RegisterLinks(container *Container, hostConfig *runconfig.HostConfig) error {
	// TODO Windows. Factored out for network modes. There may be more
	// refactoring required here.

	if hostConfig == nil || hostConfig.Links == nil {
		return nil
	}

	for _, l := range hostConfig.Links {
		name, alias, err := parsers.ParseLink(l)
		if err != nil {
			return err
		}
		child, err := daemon.Get(name)
		if err != nil {
			//An error from daemon.Get() means this name could not be found
			return fmt.Errorf("Could not get container for %s", name)
		}
		if err := daemon.RegisterLink(container, child, alias); err != nil {
			return err
		}
	}

	// After we load all the links into the daemon
	// set them to nil on the hostconfig
	hostConfig.Links = nil
	if err := container.WriteHostConfig(); err != nil {
		return err
	}
	return nil
}
