package daemon

import (
	"fmt"
	"os"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	// register the windows graph driver
	_ "github.com/docker/docker/daemon/graphdriver/windows"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
)

const (
	defaultVirtualSwitch = "Virtual Switch"
	platformSupported    = true
	windowsMinCPUShares  = 1
	windowsMaxCPUShares  = 9
)

func parseSecurityOpt(container *Container, config *runconfig.HostConfig) error {
	return nil
}

func setupInitLayer(initLayer string, rootUID, rootGID int) error {
	return nil
}

func checkKernel() error {
	return nil
}

// adaptContainerSettings is called during container creation to modify any
// settings necessary in the HostConfig structure.
func (daemon *Daemon) adaptContainerSettings(hostConfig *runconfig.HostConfig, adjustCPUShares bool) {
	if hostConfig == nil {
		return
	}

	if hostConfig.CPUShares < 0 {
		logrus.Warnf("Changing requested CPUShares of %d to minimum allowed of %d", hostConfig.CPUShares, windowsMinCPUShares)
		hostConfig.CPUShares = windowsMinCPUShares
	} else if hostConfig.CPUShares > windowsMaxCPUShares {
		logrus.Warnf("Changing requested CPUShares of %d to maximum allowed of %d", hostConfig.CPUShares, windowsMaxCPUShares)
		hostConfig.CPUShares = windowsMaxCPUShares
	}
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *runconfig.HostConfig, config *runconfig.Config) ([]string, error) {
	return nil, nil
}

// checkConfigOptions checks for mutually incompatible config options
func checkConfigOptions(config *Config) error {
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	var dwVersion uint32

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

func configureSysInit(config *Config, rootUID, rootGID int) (string, error) {
	// TODO Windows.
	return os.Getenv("TEMP"), nil
}

func isBridgeNetworkDisabled(config *Config) bool {
	return false
}

func (daemon *Daemon) initNetworkController(config *Config) (libnetwork.NetworkController, error) {
	// Set the name of the virtual switch if not specified by -b on daemon start
	if config.Bridge.VirtualSwitchName == "" {
		config.Bridge.VirtualSwitchName = defaultVirtualSwitch
	}
	return nil, nil
}

// registerLinks sets up links between containers and writes the
// configuration out for persistence.
func (daemon *Daemon) registerLinks(container *Container, hostConfig *runconfig.HostConfig) error {
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
		if err := daemon.registerLink(container, child, alias); err != nil {
			return err
		}
	}

	// After we load all the links into the daemon
	// set them to nil on the hostconfig
	hostConfig.Links = nil
	if err := container.writeHostConfig(); err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) newBaseContainer(id string) Container {
	return Container{
		CommonContainer: CommonContainer{
			ID:           id,
			State:        NewState(),
			execCommands: newExecStore(),
			root:         daemon.containerRoot(id),
		},
	}
}

func (daemon *Daemon) cleanupMounts() error {
	return nil
}
