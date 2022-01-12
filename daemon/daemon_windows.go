package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/libcontainerd/local"
	"github.com/docker/docker/libcontainerd/remote"
	"github.com/docker/docker/libnetwork"
	nwconfig "github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	winlibnetwork "github.com/docker/docker/libnetwork/drivers/windows"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/platform"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/runconfig"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	isWindows            = true
	platformSupported    = true
	windowsMinCPUShares  = 1
	windowsMaxCPUShares  = 10000
	windowsMinCPUPercent = 1
	windowsMaxCPUPercent = 100

	windowsV1RuntimeName = "com.docker.hcsshim.v1"
	windowsV2RuntimeName = "io.containerd.runhcs.v1"
)

// Windows containers are much larger than Linux containers and each of them
// have > 20 system processes which why we use much smaller parallelism value.
func adjustParallelLimit(n int, limit int) int {
	return int(math.Max(1, math.Floor(float64(runtime.NumCPU())*.8)))
}

// Windows has no concept of an execution state directory. So use config.Root here.
func getPluginExecRoot(root string) string {
	return filepath.Join(root, "plugins")
}

func (daemon *Daemon) parseSecurityOpt(container *container.Container, hostConfig *containertypes.HostConfig) error {
	return parseSecurityOpt(container, hostConfig)
}

func parseSecurityOpt(container *container.Container, config *containertypes.HostConfig) error {
	return nil
}

func setupInitLayer(idMapping *idtools.IdentityMapping) func(containerfs.ContainerFS) error {
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

	return nil
}

// verifyPlatformContainerResources performs platform-specific validation of the container's resource-configuration
func verifyPlatformContainerResources(resources *containertypes.Resources, isHyperv bool) (warnings []string, err error) {
	fixMemorySwappiness(resources)
	if !isHyperv {
		// The processor resource controls are mutually exclusive on
		// Windows Server Containers, the order of precedence is
		// CPUCount first, then CPUShares, and CPUPercent last.
		if resources.CPUCount > 0 {
			if resources.CPUShares > 0 {
				warnings = append(warnings, "Conflicting options: CPU count takes priority over CPU shares on Windows Server Containers. CPU shares discarded")
				resources.CPUShares = 0
			}
			if resources.CPUPercent > 0 {
				warnings = append(warnings, "Conflicting options: CPU count takes priority over CPU percent on Windows Server Containers. CPU percent discarded")
				resources.CPUPercent = 0
			}
		} else if resources.CPUShares > 0 {
			if resources.CPUPercent > 0 {
				warnings = append(warnings, "Conflicting options: CPU shares takes priority over CPU percent on Windows Server Containers. CPU percent discarded")
				resources.CPUPercent = 0
			}
		}
	}

	if resources.CPUShares < 0 || resources.CPUShares > windowsMaxCPUShares {
		return warnings, fmt.Errorf("range of CPUShares is from %d to %d", windowsMinCPUShares, windowsMaxCPUShares)
	}
	if resources.CPUPercent < 0 || resources.CPUPercent > windowsMaxCPUPercent {
		return warnings, fmt.Errorf("range of CPUPercent is from %d to %d", windowsMinCPUPercent, windowsMaxCPUPercent)
	}
	if resources.CPUCount < 0 {
		return warnings, fmt.Errorf("invalid CPUCount: CPUCount cannot be negative")
	}

	if resources.NanoCPUs > 0 && resources.CPUPercent > 0 {
		return warnings, fmt.Errorf("conflicting options: Nano CPUs and CPU Percent cannot both be set")
	}
	if resources.NanoCPUs > 0 && resources.CPUShares > 0 {
		return warnings, fmt.Errorf("conflicting options: Nano CPUs and CPU Shares cannot both be set")
	}
	// The precision we could get is 0.01, because on Windows we have to convert to CPUPercent.
	// We don't set the lower limit here and it is up to the underlying platform (e.g., Windows) to return an error.
	if resources.NanoCPUs < 0 || resources.NanoCPUs > int64(sysinfo.NumCPU())*1e9 {
		return warnings, fmt.Errorf("range of CPUs is from 0.01 to %d.00, as there are only %d CPUs available", sysinfo.NumCPU(), sysinfo.NumCPU())
	}

	if resources.NanoCPUs > 0 && isHyperv && osversion.Build() < osversion.RS3 {
		leftoverNanoCPUs := resources.NanoCPUs % 1e9
		if leftoverNanoCPUs != 0 && resources.NanoCPUs > 1e9 {
			resources.NanoCPUs = ((resources.NanoCPUs + 1e9/2) / 1e9) * 1e9
			warningString := fmt.Sprintf("Your current OS version does not support Hyper-V containers with NanoCPUs greater than 1000000000 but not divisible by 1000000000. NanoCPUs rounded to %d", resources.NanoCPUs)
			warnings = append(warnings, warningString)
		}
	}

	if len(resources.BlkioDeviceReadBps) > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioDeviceReadBps")
	}
	if len(resources.BlkioDeviceReadIOps) > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioDeviceReadIOps")
	}
	if len(resources.BlkioDeviceWriteBps) > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioDeviceWriteBps")
	}
	if len(resources.BlkioDeviceWriteIOps) > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioDeviceWriteIOps")
	}
	if resources.BlkioWeight > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioWeight")
	}
	if len(resources.BlkioWeightDevice) > 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support BlkioWeightDevice")
	}
	if resources.CgroupParent != "" {
		return warnings, fmt.Errorf("invalid option: Windows does not support CgroupParent")
	}
	if resources.CPUPeriod != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support CPUPeriod")
	}
	if resources.CpusetCpus != "" {
		return warnings, fmt.Errorf("invalid option: Windows does not support CpusetCpus")
	}
	if resources.CpusetMems != "" {
		return warnings, fmt.Errorf("invalid option: Windows does not support CpusetMems")
	}
	if resources.KernelMemory != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support KernelMemory")
	}
	if resources.MemoryReservation != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support MemoryReservation")
	}
	if resources.MemorySwap != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support MemorySwap")
	}
	if resources.MemorySwappiness != nil {
		return warnings, fmt.Errorf("invalid option: Windows does not support MemorySwappiness")
	}
	if resources.OomKillDisable != nil && *resources.OomKillDisable {
		return warnings, fmt.Errorf("invalid option: Windows does not support OomKillDisable")
	}
	if resources.PidsLimit != nil && *resources.PidsLimit != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support PidsLimit")
	}
	if len(resources.Ulimits) != 0 {
		return warnings, fmt.Errorf("invalid option: Windows does not support Ulimits")
	}
	return warnings, nil
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *containertypes.HostConfig, update bool) (warnings []string, err error) {
	if hostConfig == nil {
		return nil, nil
	}
	hyperv := daemon.runAsHyperVContainer(hostConfig)

	// On RS5, we allow (but don't strictly support) process isolation on Client SKUs.
	// Prior to RS5, we don't allow process isolation on Client SKUs.
	// @engine maintainers. This block should not be removed. It partially enforces licensing
	// restrictions on Windows. Ping Microsoft folks if there are concerns or PRs to change this.
	if !hyperv && system.IsWindowsClient() && osversion.Build() < osversion.RS5 {
		return warnings, fmt.Errorf("Windows client operating systems earlier than version 1809 can only run Hyper-V containers")
	}

	w, err := verifyPlatformContainerResources(&hostConfig.Resources, hyperv)
	warnings = append(warnings, w...)
	return warnings, err
}

// verifyDaemonSettings performs validation of daemon config struct
func verifyDaemonSettings(config *config.Config) error {
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	// Validate the OS version. Note that dockerd.exe must be manifested for this
	// call to return the correct version.
	if osversion.Get().MajorVersion < 10 {
		return fmt.Errorf("This version of Windows does not support the docker daemon")
	}
	if osversion.Build() < osversion.RS1 {
		return fmt.Errorf("The docker daemon requires build 14393 or later of Windows Server 2016 or Windows 10")
	}

	vmcompute := windows.NewLazySystemDLL("vmcompute.dll")
	if vmcompute.Load() != nil {
		return fmt.Errorf("failed to load vmcompute.dll, ensure that the Containers feature is installed")
	}

	// Ensure that the required Host Network Service and vmcompute services
	// are running. Docker will fail in unexpected ways if this is not present.
	var requiredServices = []string{"hns", "vmcompute"}
	if err := ensureServicesInstalled(requiredServices); err != nil {
		return errors.Wrap(err, "a required service is not installed, ensure the Containers feature is installed")
	}

	return nil
}

func ensureServicesInstalled(services []string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	for _, service := range services {
		s, err := m.OpenService(service)
		if err != nil {
			return errors.Wrapf(err, "failed to open service %s", service)
		}
		s.Close()
	}
	return nil
}

// configureKernelSecuritySupport configures and validate security support for the kernel
func configureKernelSecuritySupport(config *config.Config, driverName string) error {
	return nil
}

// configureMaxThreads sets the Go runtime max threads threshold
func configureMaxThreads(config *config.Config) error {
	return nil
}

func (daemon *Daemon) initNetworkController(config *config.Config, activeSandboxes map[string]interface{}) (libnetwork.NetworkController, error) {
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
			// global networks should not be deleted by local HNS
			if v.Info().Scope() != datastore.GlobalScope {
				err = v.Delete()
				if err != nil {
					logrus.Errorf("Error occurred when removing network %v", err)
				}
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
		networkTypeNorm := strings.ToLower(v.Type)
		if networkTypeNorm == "private" || networkTypeNorm == "internal" {
			continue // workaround for HNS reporting unsupported networks
		}
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

		drvOptions := make(map[string]string)
		nid := ""
		if n != nil {
			nid = n.ID()

			// global networks should not be deleted by local HNS
			if n.Info().Scope() == datastore.GlobalScope {
				continue
			}
			v.Name = n.Name()
			// This will not cause network delete from HNS as the network
			// is not yet populated in the libnetwork windows driver

			// restore option if it existed before
			drvOptions = n.Info().DriverOptions()
			n.Delete()
		}
		netOption := map[string]string{
			winlibnetwork.NetworkName: v.Name,
			winlibnetwork.HNSID:       v.Id,
		}

		// add persisted driver options
		for k, v := range drvOptions {
			if k != winlibnetwork.NetworkName && k != winlibnetwork.HNSID {
				netOption[k] = v
			}
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
		// encountered if it doesn't already exist
		if !defaultNetworkExists &&
			runconfig.DefaultDaemonNetworkMode() == containertypes.NetworkMode(strings.ToLower(v.Type)) &&
			n == nil {
			name = runconfig.DefaultDaemonNetworkMode().NetworkName()
			defaultNetworkExists = true
		}

		v6Conf := []*libnetwork.IpamConf{}
		_, err := controller.NewNetwork(strings.ToLower(v.Type), name, nid,
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

func initBridgeDriver(controller libnetwork.NetworkController, config *config.Config) error {
	if _, err := controller.NetworkByName(runconfig.DefaultDaemonNetworkMode().NetworkName()); err == nil {
		return nil
	}

	netOption := map[string]string{
		winlibnetwork.NetworkName: runconfig.DefaultDaemonNetworkMode().NetworkName(),
	}

	var ipamOption libnetwork.NetworkOption
	var subnetPrefix string

	if config.BridgeConfig.FixedCIDR != "" {
		subnetPrefix = config.BridgeConfig.FixedCIDR
	}

	if subnetPrefix != "" {
		ipamV4Conf := libnetwork.IpamConf{PreferredPool: subnetPrefix}
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

func recursiveUnmount(_ string) error {
	return nil
}

func setupRemappedRoot(config *config.Config) (*idtools.IdentityMapping, error) {
	return &idtools.IdentityMapping{}, nil
}

func setupDaemonRoot(config *config.Config, rootDir string, rootIdentity idtools.Identity) error {
	config.Root = rootDir
	// Create the root directory if it doesn't exists
	if err := system.MkdirAllWithACL(config.Root, 0, system.SddlAdministratorsLocalSystem); err != nil {
		return err
	}
	return nil
}

// runasHyperVContainer returns true if we are going to run as a Hyper-V container
func (daemon *Daemon) runAsHyperVContainer(hostConfig *containertypes.HostConfig) bool {
	if hostConfig.Isolation.IsDefault() {
		// Container is set to use the default, so take the default from the daemon configuration
		return daemon.defaultIsolation.IsHyperV()
	}

	// Container is requesting an isolation mode. Honour it.
	return hostConfig.Isolation.IsHyperV()

}

// conditionalMountOnStart is a platform specific helper function during the
// container start to call mount.
func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	if daemon.runAsHyperVContainer(container.HostConfig) {
		// We do not mount if a Hyper-V container as it needs to be mounted inside the
		// utility VM, not the host.
		return nil
	}
	return daemon.Mount(container)
}

// conditionalUnmountOnCleanup is a platform specific helper function called
// during the cleanup of a container to unmount.
func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) error {
	if daemon.runAsHyperVContainer(container.HostConfig) {
		// We do not unmount if a Hyper-V container
		return nil
	}
	return daemon.Unmount(container)
}

func driverOptions(_ *config.Config) nwconfig.Option {
	return nil
}

func (daemon *Daemon) stats(c *container.Container) (*types.StatsJSON, error) {
	if !c.IsRunning() {
		return nil, errNotRunning(c.ID)
	}

	// Obtain the stats from HCS via libcontainerd
	stats, err := daemon.containerd.Stats(context.Background(), c.ID)
	if err != nil {
		if strings.Contains(err.Error(), "container not found") {
			return nil, containerNotFound(c.ID)
		}
		return nil, err
	}

	// Start with an empty structure
	s := &types.StatsJSON{}
	s.Stats.Read = stats.Read
	s.Stats.NumProcs = platform.NumProcs()

	if stats.HCSStats != nil {
		hcss := stats.HCSStats
		// Populate the CPU/processor statistics
		s.CPUStats = types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage:        hcss.Processor.TotalRuntime100ns,
				UsageInKernelmode: hcss.Processor.RuntimeKernel100ns,
				UsageInUsermode:   hcss.Processor.RuntimeUser100ns,
			},
		}

		// Populate the memory statistics
		s.MemoryStats = types.MemoryStats{
			Commit:            hcss.Memory.UsageCommitBytes,
			CommitPeak:        hcss.Memory.UsageCommitPeakBytes,
			PrivateWorkingSet: hcss.Memory.UsagePrivateWorkingSetBytes,
		}

		// Populate the storage statistics
		s.StorageStats = types.StorageStats{
			ReadCountNormalized:  hcss.Storage.ReadCountNormalized,
			ReadSizeBytes:        hcss.Storage.ReadSizeBytes,
			WriteCountNormalized: hcss.Storage.WriteCountNormalized,
			WriteSizeBytes:       hcss.Storage.WriteSizeBytes,
		}

		// Populate the network statistics
		s.Networks = make(map[string]types.NetworkStats)
		for _, nstats := range hcss.Network {
			s.Networks[nstats.EndpointId] = types.NetworkStats{
				RxBytes:   nstats.BytesReceived,
				RxPackets: nstats.PacketsReceived,
				RxDropped: nstats.DroppedPacketsIncoming,
				TxBytes:   nstats.BytesSent,
				TxPackets: nstats.PacketsSent,
				TxDropped: nstats.DroppedPacketsOutgoing,
			}
		}
	}
	return s, nil
}

// setDefaultIsolation determine the default isolation mode for the
// daemon to run in. This is only applicable on Windows
func (daemon *Daemon) setDefaultIsolation() error {
	daemon.defaultIsolation = containertypes.Isolation("process")

	// On client SKUs, default to Hyper-V. @engine maintainers. This
	// should not be removed. Ping Microsoft folks is there are PRs to
	// to change this.
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
				if system.IsWindowsClient() && osversion.Build() < osversion.RS5 {
					// On RS5, we allow (but don't strictly support) process isolation on Client SKUs.
					// @engine maintainers. This block should not be removed. It partially enforces licensing
					// restrictions on Windows. Ping Microsoft folks if there are concerns or PRs to change this.
					return fmt.Errorf("Windows client operating systems earlier than version 1809 can only run Hyper-V containers")
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

func setupDaemonProcess(config *config.Config) error {
	return nil
}

func (daemon *Daemon) setupSeccompProfile() error {
	return nil
}

func (daemon *Daemon) loadRuntimes() error {
	return nil
}

func (daemon *Daemon) initRuntimes(_ map[string]types.Runtime) error {
	return nil
}

func setupResolvConf(config *config.Config) {
}

// RawSysInfo returns *sysinfo.SysInfo .
func (daemon *Daemon) RawSysInfo() *sysinfo.SysInfo {
	return sysinfo.New()
}

func (daemon *Daemon) initLibcontainerd(ctx context.Context) error {
	var err error

	rt := daemon.configStore.GetDefaultRuntimeName()
	if rt == "" {
		if daemon.configStore.ContainerdAddr == "" {
			rt = windowsV1RuntimeName
		} else {
			rt = windowsV2RuntimeName
		}
	}

	switch rt {
	case windowsV1RuntimeName:
		daemon.containerd, err = local.NewClient(
			ctx,
			daemon.containerdCli,
			filepath.Join(daemon.configStore.ExecRoot, "containerd"),
			daemon.configStore.ContainerdNamespace,
			daemon,
		)
	case windowsV2RuntimeName:
		if daemon.configStore.ContainerdAddr == "" {
			return fmt.Errorf("cannot use the specified runtime %q without containerd", rt)
		}
		daemon.containerd, err = remote.NewClient(
			ctx,
			daemon.containerdCli,
			filepath.Join(daemon.configStore.ExecRoot, "containerd"),
			daemon.configStore.ContainerdNamespace,
			daemon,
		)
	default:
		return fmt.Errorf("unknown windows runtime %s", rt)
	}

	return err
}
