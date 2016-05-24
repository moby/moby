// +build solaris,cgo

package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/docker/libnetwork"
	nwconfig "github.com/docker/libnetwork/config"
)

//#include <zone.h>
import "C"

const (
	defaultVirtualSwitch = "Virtual Switch"
	platformSupported    = true
	solarisMinCPUShares  = 1
	solarisMaxCPUShares  = 65535
)

func (daemon *Daemon) cleanupMountsByID(id string) error {
	return nil
}

func parseSecurityOpt(container *container.Container, config *containertypes.HostConfig) error {
	return nil
}

func setupRemappedRoot(config *Config) ([]idtools.IDMap, []idtools.IDMap, error) {
	return nil, nil, nil
}

func setupDaemonRoot(config *Config, rootDir string, rootUID, rootGID int) error {
	return nil
}

// setupInitLayer populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. The mountpoint is simply an
// empty file at /.dockerinit
//
// This extra layer is used by all containers as the top-most ro layer. It protects
// the container from unwanted side-effects on the rw layer.
func setupInitLayer(initLayer string, rootUID, rootGID int) error {
	return nil
}

func checkKernel() error {
	// solaris can rely upon checkSystem() below, we don't skew kernel versions
	return nil
}

func (daemon *Daemon) getCgroupDriver() string {
	return ""
}

func (daemon *Daemon) adaptContainerSettings(hostConfig *containertypes.HostConfig, adjustCPUShares bool) error {
	return nil
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *containertypes.HostConfig, config *containertypes.Config, update bool) ([]string, error) {
	warnings := []string{}
	return warnings, nil
}

// verifyDaemonSettings performs validation of daemon config struct
func verifyDaemonSettings(config *Config) error {
	// checkSystem validates platform-specific requirements
	return nil
}

func checkSystem() error {
	// check OS version for compatibility, ensure running in global zone
	var err error
	var id C.zoneid_t

	if id, err = C.getzoneid(); err != nil {
		return fmt.Errorf("Exiting. Error getting zone id: %+v", err)
	}
	if int(id) != 0 {
		return fmt.Errorf("Exiting because the Docker daemon is not running in the global zone")
	}

	v, err := kernel.GetKernelVersion()
	if kernel.CompareKernelVersion(*v, kernel.VersionInfo{Kernel: 5, Major: 12, Minor: 0}) < 0 {
		return fmt.Errorf("Your Solaris kernel version: %s doesn't support Docker. Please upgrade to 5.12.0", v.String())
	}
	return err
}

// configureMaxThreads sets the Go runtime max threads threshold
// which is 90% of the kernel setting from /proc/sys/kernel/threads-max
func configureMaxThreads(config *Config) error {
	return nil
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *Config, driverName string) error {
	return nil
}

func (daemon *Daemon) initNetworkController(config *Config) (libnetwork.NetworkController, error) {
	return nil, nil
}

// registerLinks sets up links between containers and writes the
// configuration out for persistence.
func (daemon *Daemon) registerLinks(container *container.Container, hostConfig *containertypes.HostConfig) error {
	return nil
}

func (daemon *Daemon) cleanupMounts() error {
	return nil
}

// conditionalMountOnStart is a platform specific helper function during the
// container start to call mount.
func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	return nil
}

// conditionalUnmountOnCleanup is a platform specific helper function called
// during the cleanup of a container to unmount.
func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) error {
	return daemon.Unmount(container)
}

func restoreCustomImage(is image.Store, ls layer.Store, rs reference.Store) error {
	// Solaris has no custom images to register
	return nil
}

func driverOptions(config *Config) []nwconfig.Option {
	return []nwconfig.Option{}
}

func (daemon *Daemon) stats(c *container.Container) (*types.StatsJSON, error) {
	return nil, nil
}

// setDefaultIsolation determine the default isolation mode for the
// daemon to run in. This is only applicable on Windows
func (daemon *Daemon) setDefaultIsolation() error {
	return nil
}

func rootFSToAPIType(rootfs *image.RootFS) types.RootFS {
	return types.RootFS{}
}
