package daemon

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/parsers/operatingsystem"
	"github.com/docker/docker/pkg/platform"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volume/drivers"
)

// SystemInfo returns information about the host server the daemon is running on.
func (daemon *Daemon) SystemInfo() (*types.Info, error) {
	kernelVersion := "<unknown>"
	if kv, err := kernel.GetKernelVersion(); err == nil {
		kernelVersion = kv.String()
	}

	operatingSystem := "<unknown>"
	if s, err := operatingsystem.GetOperatingSystem(); err == nil {
		operatingSystem = s
	}

	// Don't do containerized check on Windows
	if runtime.GOOS != "windows" {
		if inContainer, err := operatingsystem.IsContainerized(); err != nil {
			logrus.Errorf("Could not determine if daemon is containerized: %v", err)
			operatingSystem += " (error determining if containerized)"
		} else if inContainer {
			operatingSystem += " (containerized)"
		}
	}

	meminfo, err := system.ReadMemInfo()
	if err != nil {
		logrus.Errorf("Could not read system memory info: %v", err)
	}

	// if we still have the original dockerinit binary from before
	// we copied it locally, let's return the path to that, since
	// that's more intuitive (the copied path is trivial to derive
	// by hand given VERSION)
	initPath := utils.DockerInitPath("")
	sysInfo := sysinfo.New(true)

	v := &types.Info{
		ID:                 daemon.ID,
		Containers:         len(daemon.List()),
		Images:             len(daemon.imageStore.Map()),
		Driver:             daemon.GraphDriver().String(),
		DriverStatus:       daemon.GraphDriver().Status(),
		Plugins:            daemon.showPluginsInfo(),
		IPv4Forwarding:     !sysInfo.IPv4ForwardingDisabled,
		BridgeNfIptables:   !sysInfo.BridgeNfCallIptablesDisabled,
		BridgeNfIP6tables:  !sysInfo.BridgeNfCallIP6tablesDisabled,
		Debug:              os.Getenv("DEBUG") != "",
		NFd:                fileutils.GetTotalUsedFds(),
		NGoroutines:        runtime.NumGoroutine(),
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		ExecutionDriver:    daemon.ExecutionDriver().Name(),
		LoggingDriver:      daemon.defaultLogConfig.Type,
		NEventsListener:    daemon.EventsService.SubscribersCount(),
		KernelVersion:      kernelVersion,
		OperatingSystem:    operatingSystem,
		IndexServerAddress: registry.IndexServer,
		OSType:             platform.OSType,
		Architecture:       platform.Architecture,
		RegistryConfig:     daemon.RegistryService.Config,
		InitSha1:           dockerversion.InitSHA1,
		InitPath:           initPath,
		NCPU:               runtime.NumCPU(),
		MemTotal:           meminfo.MemTotal,
		DockerRootDir:      daemon.config().Root,
		Labels:             daemon.config().Labels,
		ExperimentalBuild:  utils.ExperimentalBuild(),
		ServerVersion:      dockerversion.Version,
		ClusterStore:       daemon.config().ClusterStore,
		ClusterAdvertise:   daemon.config().ClusterAdvertise,
		HTTPProxy:          getProxyEnv("http_proxy"),
		HTTPSProxy:         getProxyEnv("https_proxy"),
		NoProxy:            getProxyEnv("no_proxy"),
	}

	// TODO Windows. Refactor this more once sysinfo is refactored into
	// platform specific code. On Windows, sysinfo.cgroupMemInfo and
	// sysinfo.cgroupCpuInfo will be nil otherwise and cause a SIGSEGV if
	// an attempt is made to access through them.
	if runtime.GOOS != "windows" {
		v.MemoryLimit = sysInfo.MemoryLimit
		v.SwapLimit = sysInfo.SwapLimit
		v.OomKillDisable = sysInfo.OomKillDisable
		v.CPUCfsPeriod = sysInfo.CPUCfsPeriod
		v.CPUCfsQuota = sysInfo.CPUCfsQuota
		v.CPUShares = sysInfo.CPUShares
		v.CPUSet = sysInfo.Cpuset
	}

	if hostname, err := os.Hostname(); err == nil {
		v.Name = hostname
	}

	return v, nil
}

func (daemon *Daemon) showPluginsInfo() types.PluginsInfo {
	var pluginsInfo types.PluginsInfo

	pluginsInfo.Volume = volumedrivers.GetDriverList()

	networkDriverList := daemon.GetNetworkDriverList()
	for nd := range networkDriverList {
		pluginsInfo.Network = append(pluginsInfo.Network, nd)
	}

	return pluginsInfo
}

// The uppercase and the lowercase are available for the proxy settings.
// See the Go specification for details on these variables. https://golang.org/pkg/net/http/
func getProxyEnv(key string) string {
	proxyValue := os.Getenv(strings.ToUpper(key))
	if proxyValue == "" {
		return os.Getenv(strings.ToLower(key))
	}
	return proxyValue
}
