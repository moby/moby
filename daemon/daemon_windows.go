package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
	containertypes "github.com/docker/engine-api/types/container"
	// register the windows graph driver
	"github.com/docker/docker/daemon/graphdriver/windows"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/libnetwork"
	nwconfig "github.com/docker/libnetwork/config"
	blkiodev "github.com/opencontainers/runc/libcontainer/configs"
)

const (
	defaultVirtualSwitch = "Virtual Switch"
	platformSupported    = true
	windowsMinCPUShares  = 1
	windowsMaxCPUShares  = 10000
)

func getBlkioWeightDevices(config *containertypes.HostConfig) ([]*blkiodev.WeightDevice, error) {
	return nil, nil
}

func parseSecurityOpt(container *container.Container, config *containertypes.HostConfig) error {
	return nil
}

func getBlkioReadIOpsDevices(config *containertypes.HostConfig) ([]*blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioWriteIOpsDevices(config *containertypes.HostConfig) ([]*blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioReadBpsDevices(config *containertypes.HostConfig) ([]*blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioWriteBpsDevices(config *containertypes.HostConfig) ([]*blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func setupInitLayer(initLayer string, rootUID, rootGID int) error {
	return nil
}

func checkKernel() error {
	return nil
}

func (daemon *Daemon) getCgroupDriver() string {
	return ""
}

// adaptContainerSettings is called during container creation to modify any
// settings necessary in the HostConfig structure.
func (daemon *Daemon) adaptContainerSettings(hostConfig *containertypes.HostConfig, adjustCPUShares bool) error {
	if hostConfig == nil {
		return nil
	}

	if hostConfig.CPUShares < 0 {
		logrus.Warnf("Changing requested CPUShares of %d to minimum allowed of %d", hostConfig.CPUShares, windowsMinCPUShares)
		hostConfig.CPUShares = windowsMinCPUShares
	} else if hostConfig.CPUShares > windowsMaxCPUShares {
		logrus.Warnf("Changing requested CPUShares of %d to maximum allowed of %d", hostConfig.CPUShares, windowsMaxCPUShares)
		hostConfig.CPUShares = windowsMaxCPUShares
	}

	return nil
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *containertypes.HostConfig, config *containertypes.Config, update bool) ([]string, error) {
	return nil, nil
}

// verifyDaemonSettings performs validation of daemon config struct
func verifyDaemonSettings(config *Config) error {
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	// Validate the OS version. Note that docker.exe must be manifested for this
	// call to return the correct version.
	osv, err := system.GetOSVersion()
	if err != nil {
		return err
	}
	if osv.MajorVersion < 10 {
		return fmt.Errorf("This version of Windows does not support the docker daemon")
	}
	if osv.Build < 10586 {
		return fmt.Errorf("The Windows daemon requires Windows Server 2016 Technical Preview 4, build 10586 or later")
	}
	return nil
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *Config, driverName string) error {
	return nil
}

// configureMaxThreads sets the Go runtime max threads threshold
func configureMaxThreads(config *Config) error {
	return nil
}

func isBridgeNetworkDisabled(config *Config) bool {
	return false
}

func (daemon *Daemon) initNetworkController(config *Config) (libnetwork.NetworkController, error) {
	// Set the name of the virtual switch if not specified by -b on daemon start
	if config.bridgeConfig.VirtualSwitchName == "" {
		config.bridgeConfig.VirtualSwitchName = defaultVirtualSwitch
	}
	return nil, nil
}

// registerLinks sets up links between containers and writes the
// configuration out for persistence. As of Windows TP4, links are not supported.
func (daemon *Daemon) registerLinks(container *container.Container, hostConfig *containertypes.HostConfig) error {
	return nil
}

func (daemon *Daemon) cleanupMounts() error {
	return nil
}

func setupRemappedRoot(config *Config) ([]idtools.IDMap, []idtools.IDMap, error) {
	return nil, nil, nil
}

func setupDaemonRoot(config *Config, rootDir string, rootUID, rootGID int) error {
	config.Root = rootDir
	// Create the root directory if it doesn't exists
	if err := system.MkdirAll(config.Root, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

// conditionalMountOnStart is a platform specific helper function during the
// container start to call mount.
func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	// We do not mount if a Hyper-V container
	if !container.HostConfig.Isolation.IsHyperV() {
		if err := daemon.Mount(container); err != nil {
			return err
		}
	}
	return nil
}

// conditionalUnmountOnCleanup is a platform specific helper function called
// during the cleanup of a container to unmount.
func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) {
	// We do not unmount if a Hyper-V container
	if !container.HostConfig.Isolation.IsHyperV() {
		daemon.Unmount(container)
	}
}

func restoreCustomImage(is image.Store, ls layer.Store, rs reference.Store) error {
	type graphDriverStore interface {
		GraphDriver() graphdriver.Driver
	}

	gds, ok := ls.(graphDriverStore)
	if !ok {
		return nil
	}

	driver := gds.GraphDriver()
	wd, ok := driver.(*windows.Driver)
	if !ok {
		return nil
	}

	imageInfos, err := wd.GetCustomImageInfos()
	if err != nil {
		return err
	}

	// Convert imageData to valid image configuration
	for i := range imageInfos {
		name := strings.ToLower(imageInfos[i].Name)

		type registrar interface {
			RegisterDiffID(graphID string, size int64) (layer.Layer, error)
		}
		r, ok := ls.(registrar)
		if !ok {
			return errors.New("Layerstore doesn't support RegisterDiffID")
		}
		if _, err := r.RegisterDiffID(imageInfos[i].ID, imageInfos[i].Size); err != nil {
			return err
		}
		// layer is intentionally not released

		rootFS := image.NewRootFS()
		rootFS.BaseLayer = filepath.Base(imageInfos[i].Path)

		// Create history for base layer
		config, err := json.Marshal(&image.Image{
			V1Image: image.V1Image{
				DockerVersion: dockerversion.Version,
				Architecture:  runtime.GOARCH,
				OS:            runtime.GOOS,
				Created:       imageInfos[i].CreatedTime,
			},
			RootFS:  rootFS,
			History: []image.History{},
		})

		named, err := reference.ParseNamed(name)
		if err != nil {
			return err
		}

		ref, err := reference.WithTag(named, imageInfos[i].Version)
		if err != nil {
			return err
		}

		id, err := is.Create(config)
		if err != nil {
			return err
		}

		if err := rs.AddTag(ref, id, true); err != nil {
			return err
		}

		logrus.Debugf("Registered base layer %s as %s", ref, id)
	}
	return nil
}

func (daemon *Daemon) networkOptions(dconfig *Config) ([]nwconfig.Option, error) {
	return nil, fmt.Errorf("Network controller config reload not aavailable on Windows yet")
}
