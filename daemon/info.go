package daemon

import (
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
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
	"github.com/docker/engine-api/types"
	"github.com/docker/go-connections/sockets"
)

// SystemInfo returns information about the host server the daemon is running on.
func (daemon *Daemon) SystemInfo() (*types.Info, error) {
	kernelVersion := "<unknown>"
	if kv, err := kernel.GetKernelVersion(); err != nil {
		logrus.Warnf("Could not get kernel version: %v", err)
	} else {
		kernelVersion = kv.String()
	}

	operatingSystem := "<unknown>"
	if s, err := operatingsystem.GetOperatingSystem(); err != nil {
		logrus.Warnf("Could not get operating system name: %v", err)
	} else {
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

	sysInfo := sysinfo.New(true)

	var cRunning, cPaused, cStopped int32
	daemon.containers.ApplyAll(func(c *container.Container) {
		switch c.StateString() {
		case "paused":
			atomic.AddInt32(&cPaused, 1)
		case "running":
			atomic.AddInt32(&cRunning, 1)
		default:
			atomic.AddInt32(&cStopped, 1)
		}
	})

	var securityOptions []string
	if sysInfo.AppArmor {
		securityOptions = append(securityOptions, "apparmor")
	}
	if sysInfo.Seccomp {
		securityOptions = append(securityOptions, "seccomp")
	}
	if selinuxEnabled() {
		securityOptions = append(securityOptions, "selinux")
	}

	v := &types.Info{
		ID:                 daemon.ID,
		Containers:         int(cRunning + cPaused + cStopped),
		ContainersRunning:  int(cRunning),
		ContainersPaused:   int(cPaused),
		ContainersStopped:  int(cStopped),
		Images:             len(daemon.imageStore.Map()),
		Driver:             daemon.GraphDriverName(),
		DriverStatus:       daemon.layerStore.DriverStatus(),
		Plugins:            daemon.showPluginsInfo(),
		IPv4Forwarding:     !sysInfo.IPv4ForwardingDisabled,
		BridgeNfIptables:   !sysInfo.BridgeNFCallIPTablesDisabled,
		BridgeNfIP6tables:  !sysInfo.BridgeNFCallIP6TablesDisabled,
		Debug:              utils.IsDebugEnabled(),
		NFd:                fileutils.GetTotalUsedFds(),
		NGoroutines:        runtime.NumGoroutine(),
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		LoggingDriver:      daemon.defaultLogConfig.Type,
		CgroupDriver:       daemon.getCgroupDriver(),
		NEventsListener:    daemon.EventsService.SubscribersCount(),
		KernelVersion:      kernelVersion,
		OperatingSystem:    operatingSystem,
		IndexServerAddress: registry.IndexServer,
		OSType:             platform.OSType,
		Architecture:       platform.Architecture,
		RegistryConfig:     daemon.RegistryService.ServiceConfig(),
		NCPU:               runtime.NumCPU(),
		MemTotal:           meminfo.MemTotal,
		DockerRootDir:      daemon.configStore.Root,
		Labels:             daemon.configStore.Labels,
		ExperimentalBuild:  utils.ExperimentalBuild(),
		ServerVersion:      dockerversion.Version,
		ClusterStore:       daemon.configStore.ClusterStore,
		ClusterAdvertise:   daemon.configStore.ClusterAdvertise,
		HTTPProxy:          sockets.GetProxyEnv("http_proxy"),
		HTTPSProxy:         sockets.GetProxyEnv("https_proxy"),
		NoProxy:            sockets.GetProxyEnv("no_proxy"),
		SecurityOptions:    securityOptions,
	}

	// TODO Windows. Refactor this more once sysinfo is refactored into
	// platform specific code. On Windows, sysinfo.cgroupMemInfo and
	// sysinfo.cgroupCpuInfo will be nil otherwise and cause a SIGSEGV if
	// an attempt is made to access through them.
	if runtime.GOOS != "windows" {
		v.MemoryLimit = sysInfo.MemoryLimit
		v.SwapLimit = sysInfo.SwapLimit
		v.KernelMemory = sysInfo.KernelMemory
		v.OomKillDisable = sysInfo.OomKillDisable
		v.CPUCfsPeriod = sysInfo.CPUCfsPeriod
		v.CPUCfsQuota = sysInfo.CPUCfsQuota
		v.CPUShares = sysInfo.CPUShares
		v.CPUSet = sysInfo.Cpuset
	}

	hostname := ""
	if hn, err := os.Hostname(); err != nil {
		logrus.Warnf("Could not get hostname: %v", err)
	} else {
		hostname = hn
	}
	v.Name = hostname

	return v, nil
}

// SystemVersion returns version information about the daemon.
func (daemon *Daemon) SystemVersion() types.Version {
	v := types.Version{
		Version:      dockerversion.Version,
		GitCommit:    dockerversion.GitCommit,
		GoVersion:    runtime.Version(),
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		BuildTime:    dockerversion.BuildTime,
		Experimental: utils.ExperimentalBuild(),
	}

	kernelVersion := "<unknown>"
	if kv, err := kernel.GetKernelVersion(); err != nil {
		logrus.Warnf("Could not get kernel version: %v", err)
	} else {
		kernelVersion = kv.String()
	}
	v.KernelVersion = kernelVersion

	return v
}

func (daemon *Daemon) showPluginsInfo() types.PluginsInfo {
	var pluginsInfo types.PluginsInfo

	pluginsInfo.Volume = volumedrivers.GetDriverList()

	networkDriverList := daemon.GetNetworkDriverList()
	for nd := range networkDriverList {
		pluginsInfo.Network = append(pluginsInfo.Network, nd)
	}

	pluginsInfo.Authorization = daemon.configStore.AuthorizationPlugins

	return pluginsInfo
}
