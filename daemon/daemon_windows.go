package daemon

import (
	"fmt"
	"os"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	pblkiodev "github.com/docker/docker/api/types/blkiodev"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/platform"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork"
	nwconfig "github.com/docker/libnetwork/config"
	winlibnetwork "github.com/docker/libnetwork/drivers/windows"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	blkiodev "github.com/opencontainers/runc/libcontainer/configs"
)

const (
	defaultNetworkSpace = "172.16.0.0/12"
	platformSupported   = true
	windowsMinCPUShares = 1
	windowsMaxCPUShares = 10000
)

func getBlkioWeightDevices(config *containertypes.HostConfig) ([]blkiodev.WeightDevice, error) {
	return nil, nil
}

func parseSecurityOpt(container *container.Container, config *containertypes.HostConfig) error {
	return nil
}

func getBlkioReadIOpsDevices(config *containertypes.HostConfig) ([]blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioWriteIOpsDevices(config *containertypes.HostConfig) ([]blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioReadBpsDevices(config *containertypes.HostConfig) ([]blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func getBlkioWriteBpsDevices(config *containertypes.HostConfig) ([]blkiodev.ThrottleDevice, error) {
	return nil, nil
}

func setupInitLayer(initLayer string, rootUID, rootGID int) error {
	return nil
}

func (daemon *Daemon) getLayerInit() func(string) error {
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

func verifyContainerResources(resources *containertypes.Resources, sysInfo *sysinfo.SysInfo) ([]string, error) {
	warnings := []string{}

	// cpu subsystem checks and adjustments
	if resources.CPUPercent < 0 || resources.CPUPercent > 100 {
		return warnings, fmt.Errorf("Range of CPU percent is from 1 to 100")
	}

	if resources.CPUPercent > 0 && resources.CPUShares > 0 {
		return warnings, fmt.Errorf("Conflicting options: CPU Shares and CPU Percent cannot both be set")
	}

	// TODO Windows: Add more validation of resource settings not supported on Windows

	if resources.BlkioWeight > 0 {
		warnings = append(warnings, "Windows does not support Block I/O weight. Block I/O weight discarded.")
		logrus.Warn("Windows does not support Block I/O weight. Block I/O weight discarded.")
		resources.BlkioWeight = 0
	}
	if len(resources.BlkioWeightDevice) > 0 {
		warnings = append(warnings, "Windows does not support Block I/O weight-device. Weight-device discarded.")
		logrus.Warn("Windows does not support Block I/O weight-device. Weight-device discarded.")
		resources.BlkioWeightDevice = []*pblkiodev.WeightDevice{}
	}
	if len(resources.BlkioDeviceReadBps) > 0 {
		warnings = append(warnings, "Windows does not support Block read limit in bytes per second. Device read bps discarded.")
		logrus.Warn("Windows does not support Block I/O read limit in bytes per second. Device read bps discarded.")
		resources.BlkioDeviceReadBps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceWriteBps) > 0 {
		warnings = append(warnings, "Windows does not support Block write limit in bytes per second. Device write bps discarded.")
		logrus.Warn("Windows does not support Block I/O write limit in bytes per second. Device write bps discarded.")
		resources.BlkioDeviceWriteBps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceReadIOps) > 0 {
		warnings = append(warnings, "Windows does not support Block read limit in IO per second. Device read iops discarded.")
		logrus.Warn("Windows does not support Block I/O read limit in IO per second. Device read iops discarded.")
		resources.BlkioDeviceReadIOps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceWriteIOps) > 0 {
		warnings = append(warnings, "Windows does not support Block write limit in IO per second. Device write iops discarded.")
		logrus.Warn("Windows does not support Block I/O write limit in IO per second. Device write iops discarded.")
		resources.BlkioDeviceWriteIOps = []*pblkiodev.ThrottleDevice{}
	}
	return warnings, nil
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *containertypes.HostConfig, config *containertypes.Config, update bool) ([]string, error) {
	warnings := []string{}

	w, err := verifyContainerResources(&hostConfig.Resources, nil)
	warnings = append(warnings, w...)
	if err != nil {
		return warnings, err
	}

	return warnings, nil
}

// platformReload update configuration with platform specific options
func (daemon *Daemon) platformReload(config *Config) map[string]string {
	return map[string]string{}
}

// verifyDaemonSettings performs validation of daemon config struct
func verifyDaemonSettings(config *Config) error {
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	// Validate the OS version. Note that docker.exe must be manifested for this
	// call to return the correct version.
	osv := system.GetOSVersion()
	if osv.MajorVersion < 10 {
		return fmt.Errorf("This version of Windows does not support the docker daemon")
	}
	if osv.Build < 14393 {
		return fmt.Errorf("The docker daemon requires build 14393 or later of Windows Server 2016 or Windows 10")
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

func (daemon *Daemon) initNetworkController(config *Config, activeSandboxes map[string]interface{}) (libnetwork.NetworkController, error) {
	netOptions, err := daemon.networkOptions(config, nil, nil)
	if err != nil {
		return nil, err
	}
	controller, err := libnetwork.New(netOptions...)
	if err != nil {
		return nil, fmt.Errorf("error obtaining controller instance: %v", err)
	}

	hnsresponse, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return nil, err
	}

	// Remove networks not present in HNS
	for _, v := range controller.Networks() {
		options := v.Info().DriverOptions()
		hnsid := options[winlibnetwork.HNSID]
		found := false

		for _, v := range hnsresponse {
			if v.Id == hnsid {
				found = true
				break
			}
		}

		if !found {
			err = v.Delete()
			if err != nil {
				return nil, err
			}
		}
	}

	_, err = controller.NewNetwork("null", "none", "", libnetwork.NetworkOptionPersist(false))
	if err != nil {
		return nil, err
	}

	defaultNetworkExists := false

	if network, err := controller.NetworkByName(runconfig.DefaultDaemonNetworkMode().NetworkName()); err == nil {
		options := network.Info().DriverOptions()
		for _, v := range hnsresponse {
			if options[winlibnetwork.HNSID] == v.Id {
				defaultNetworkExists = true
				break
			}
		}
	}

	// discover and add HNS networks to windows
	// network that exist are removed and added again
	for _, v := range hnsresponse {
		var n libnetwork.Network
		s := func(current libnetwork.Network) bool {
			options := current.Info().DriverOptions()
			if options[winlibnetwork.HNSID] == v.Id {
				n = current
				return true
			}
			return false
		}

		controller.WalkNetworks(s)
		if n != nil {
			v.Name = n.Name()
			// This will not cause network delete from HNS as the network
			// is not yet populated in the libnetwork windows driver
			n.Delete()
		}

		netOption := map[string]string{
			winlibnetwork.NetworkName: v.Name,
			winlibnetwork.HNSID:       v.Id,
		}

		v4Conf := []*libnetwork.IpamConf{}
		for _, subnet := range v.Subnets {
			ipamV4Conf := libnetwork.IpamConf{}
			ipamV4Conf.PreferredPool = subnet.AddressPrefix
			ipamV4Conf.Gateway = subnet.GatewayAddress
			v4Conf = append(v4Conf, &ipamV4Conf)
		}

		name := v.Name

		// If there is no nat network create one from the first NAT network
		// encountered
		if !defaultNetworkExists && runconfig.DefaultDaemonNetworkMode() == containertypes.NetworkMode(strings.ToLower(v.Type)) {
			name = runconfig.DefaultDaemonNetworkMode().NetworkName()
			defaultNetworkExists = true
		}

		v6Conf := []*libnetwork.IpamConf{}
		_, err := controller.NewNetwork(strings.ToLower(v.Type), name, "",
			libnetwork.NetworkOptionGeneric(options.Generic{
				netlabel.GenericData: netOption,
			}),
			libnetwork.NetworkOptionIpam("default", "", v4Conf, v6Conf, nil),
		)

		if err != nil {
			logrus.Errorf("Error occurred when creating network %v", err)
		}
	}

	if !config.DisableBridge {
		// Initialize default driver "bridge"
		if err := initBridgeDriver(controller, config); err != nil {
			return nil, err
		}
	}

	return controller, nil
}

func initBridgeDriver(controller libnetwork.NetworkController, config *Config) error {
	if _, err := controller.NetworkByName(runconfig.DefaultDaemonNetworkMode().NetworkName()); err == nil {
		return nil
	}

	netOption := map[string]string{
		winlibnetwork.NetworkName: runconfig.DefaultDaemonNetworkMode().NetworkName(),
	}

	var ipamOption libnetwork.NetworkOption
	var subnetPrefix string

	if config.bridgeConfig.FixedCIDR != "" {
		subnetPrefix = config.bridgeConfig.FixedCIDR
	} else {
		// TP5 doesn't support properly detecting subnet
		osv := system.GetOSVersion()
		if osv.Build < 14360 {
			subnetPrefix = defaultNetworkSpace
		}
	}

	if subnetPrefix != "" {
		ipamV4Conf := libnetwork.IpamConf{}
		ipamV4Conf.PreferredPool = subnetPrefix
		v4Conf := []*libnetwork.IpamConf{&ipamV4Conf}
		v6Conf := []*libnetwork.IpamConf{}
		ipamOption = libnetwork.NetworkOptionIpam("default", "", v4Conf, v6Conf, nil)
	}

	_, err := controller.NewNetwork(string(runconfig.DefaultDaemonNetworkMode()), runconfig.DefaultDaemonNetworkMode().NetworkName(), "",
		libnetwork.NetworkOptionGeneric(options.Generic{
			netlabel.GenericData: netOption,
		}),
		ipamOption,
	)

	if err != nil {
		return fmt.Errorf("Error creating default network: %v", err)
	}

	return nil
}

// registerLinks sets up links between containers and writes the
// configuration out for persistence. As of Windows TP4, links are not supported.
func (daemon *Daemon) registerLinks(container *container.Container, hostConfig *containertypes.HostConfig) error {
	return nil
}

func (daemon *Daemon) cleanupMountsByID(in string) error {
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

// runasHyperVContainer returns true if we are going to run as a Hyper-V container
func (daemon *Daemon) runAsHyperVContainer(container *container.Container) bool {
	if container.HostConfig.Isolation.IsDefault() {
		// Container is set to use the default, so take the default from the daemon configuration
		return daemon.defaultIsolation.IsHyperV()
	}

	// Container is requesting an isolation mode. Honour it.
	return container.HostConfig.Isolation.IsHyperV()

}

// conditionalMountOnStart is a platform specific helper function during the
// container start to call mount.
func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	// We do not mount if a Hyper-V container
	if !daemon.runAsHyperVContainer(container) {
		return daemon.Mount(container)
	}
	return nil
}

// conditionalUnmountOnCleanup is a platform specific helper function called
// during the cleanup of a container to unmount.
func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) error {
	// We do not unmount if a Hyper-V container
	if !daemon.runAsHyperVContainer(container) {
		return daemon.Unmount(container)
	}
	return nil
}

func driverOptions(config *Config) []nwconfig.Option {
	return []nwconfig.Option{}
}

func (daemon *Daemon) stats(c *container.Container) (*types.StatsJSON, error) {
	if !c.IsRunning() {
		return nil, errNotRunning{c.ID}
	}

	// Obtain the stats from HCS via libcontainerd
	stats, err := daemon.containerd.Stats(c.ID)
	if err != nil {
		return nil, err
	}

	// Start with an empty structure
	s := &types.StatsJSON{}

	// Populate the CPU/processor statistics
	s.CPUStats = types.CPUStats{
		CPUUsage: types.CPUUsage{
			TotalUsage:        stats.Processor.TotalRuntime100ns,
			UsageInKernelmode: stats.Processor.RuntimeKernel100ns,
			UsageInUsermode:   stats.Processor.RuntimeKernel100ns,
		},
	}

	// Populate the memory statistics
	s.MemoryStats = types.MemoryStats{
		Commit:            stats.Memory.UsageCommitBytes,
		CommitPeak:        stats.Memory.UsageCommitPeakBytes,
		PrivateWorkingSet: stats.Memory.UsagePrivateWorkingSetBytes,
	}

	// Populate the storage statistics
	s.StorageStats = types.StorageStats{
		ReadCountNormalized:  stats.Storage.ReadCountNormalized,
		ReadSizeBytes:        stats.Storage.ReadSizeBytes,
		WriteCountNormalized: stats.Storage.WriteCountNormalized,
		WriteSizeBytes:       stats.Storage.WriteSizeBytes,
	}

	// Populate the network statistics
	s.Networks = make(map[string]types.NetworkStats)

	for _, nstats := range stats.Network {
		s.Networks[nstats.EndpointId] = types.NetworkStats{
			RxBytes:   nstats.BytesReceived,
			RxPackets: nstats.PacketsReceived,
			RxDropped: nstats.DroppedPacketsIncoming,
			TxBytes:   nstats.BytesSent,
			TxPackets: nstats.PacketsSent,
			TxDropped: nstats.DroppedPacketsOutgoing,
		}
	}

	// Set the timestamp
	s.Stats.Read = stats.Timestamp
	s.Stats.NumProcs = platform.NumProcs()

	return s, nil
}

// setDefaultIsolation determine the default isolation mode for the
// daemon to run in. This is only applicable on Windows
func (daemon *Daemon) setDefaultIsolation() error {
	daemon.defaultIsolation = containertypes.Isolation("process")
	// On client SKUs, default to Hyper-V
	if system.IsWindowsClient() {
		daemon.defaultIsolation = containertypes.Isolation("hyperv")
	}
	for _, option := range daemon.configStore.ExecOptions {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return err
		}
		key = strings.ToLower(key)
		switch key {

		case "isolation":
			if !containertypes.Isolation(val).IsValid() {
				return fmt.Errorf("Invalid exec-opt value for 'isolation':'%s'", val)
			}
			if containertypes.Isolation(val).IsHyperV() {
				daemon.defaultIsolation = containertypes.Isolation("hyperv")
			}
			if containertypes.Isolation(val).IsProcess() {
				if system.IsWindowsClient() {
					return fmt.Errorf("Windows client operating systems only support Hyper-V containers")
				}
				daemon.defaultIsolation = containertypes.Isolation("process")
			}
		default:
			return fmt.Errorf("Unrecognised exec-opt '%s'\n", key)
		}
	}

	logrus.Infof("Windows default isolation mode: %s", daemon.defaultIsolation)
	return nil
}

func rootFSToAPIType(rootfs *image.RootFS) types.RootFS {
	var layers []string
	for _, l := range rootfs.DiffIDs {
		layers = append(layers, l.String())
	}
	return types.RootFS{
		Type:   rootfs.Type,
		Layers: layers,
	}
}

func setupDaemonProcess(config *Config) error {
	return nil
}

// verifyVolumesInfo is a no-op on windows.
// This is called during daemon initialization to migrate volumes from pre-1.7.
// volumes were not supported on windows pre-1.7
func (daemon *Daemon) verifyVolumesInfo(container *container.Container) error {
	return nil
}
