package daemon

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/pkg/tracing"
	"github.com/containerd/log"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/command/debug"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/filedescriptors"
	"github.com/moby/moby/v2/daemon/internal/metrics"
	"github.com/moby/moby/v2/daemon/internal/platform"
	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/moby/moby/v2/dockerversion"
	"github.com/moby/moby/v2/pkg/meminfo"
	"github.com/moby/moby/v2/pkg/parsers/kernel"
	"github.com/moby/moby/v2/pkg/parsers/operatingsystem"
	"github.com/moby/moby/v2/pkg/sysinfo"
	"github.com/opencontainers/selinux/go-selinux"
)

func doWithTrace[T any](ctx context.Context, name string, f func() T) T {
	_, span := tracing.StartSpan(ctx, name)
	defer span.End()
	return f()
}

// SystemInfo returns information about the host server the daemon is running on.
//
// The only error this should return is due to context cancellation/deadline.
// Anything else should be logged and ignored because this is looking up
// multiple things and is often used for debugging.
// The only case valid early return is when the caller doesn't want the result anymore (ie context cancelled).
func (daemon *Daemon) SystemInfo(ctx context.Context) (*system.Info, error) {
	defer metrics.StartTimer(metrics.HostInfoFunctions.WithValues("system_info"))()

	sysInfo := daemon.RawSysInfo()
	cfg := daemon.config()

	v := &system.Info{
		ID:                 daemon.id,
		Images:             daemon.imageService.CountImages(ctx),
		IPv4Forwarding:     !sysInfo.IPv4ForwardingDisabled,
		Name:               hostName(ctx),
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		LoggingDriver:      daemon.defaultLogConfig.Type,
		KernelVersion:      kernelVersion(ctx),
		OperatingSystem:    operatingSystem(ctx),
		OSVersion:          osVersion(ctx),
		IndexServerAddress: registry.IndexServer,
		OSType:             runtime.GOOS,
		Architecture:       platform.Architecture(),
		RegistryConfig:     doWithTrace(ctx, "registry.ServiceConfig", daemon.registryService.ServiceConfig),
		NCPU:               doWithTrace(ctx, "runtime.NumCPU", runtime.NumCPU),
		MemTotal:           memInfo(ctx).MemTotal,
		GenericResources:   daemon.genericResources,
		DockerRootDir:      cfg.Root,
		Labels:             cfg.Labels,
		ExperimentalBuild:  cfg.Experimental,
		ServerVersion:      dockerversion.Version,
		HTTPProxy:          config.MaskCredentials(getConfigOrEnv(cfg.HTTPProxy, "HTTP_PROXY", "http_proxy")),
		HTTPSProxy:         config.MaskCredentials(getConfigOrEnv(cfg.HTTPSProxy, "HTTPS_PROXY", "https_proxy")),
		NoProxy:            getConfigOrEnv(cfg.NoProxy, "NO_PROXY", "no_proxy"),
		LiveRestoreEnabled: cfg.LiveRestoreEnabled,
		Isolation:          daemon.defaultIsolation,
		CDISpecDirs:        promoteNil(cfg.CDISpecDirs),
		NRI:                daemon.nri.GetInfo(),
	}

	daemon.fillContainerStates(v)
	daemon.fillDebugInfo(ctx, v)
	daemon.fillContainerdInfo(v, &cfg.Config)
	daemon.fillAPIInfo(v, &cfg.Config)

	// Retrieve platform specific info
	if err := daemon.fillPlatformInfo(ctx, v, sysInfo, cfg); err != nil {
		return nil, err
	}
	daemon.fillDriverInfo(v)
	daemon.fillPluginsInfo(ctx, v, &cfg.Config)
	daemon.fillSecurityOptions(v, sysInfo, &cfg.Config)
	daemon.fillLicense(v)
	daemon.fillDefaultAddressPools(ctx, v, &cfg.Config)
	daemon.fillFirewallInfo(v)
	daemon.fillDiscoveredDevicesFromDrivers(ctx, v, &cfg.Config)

	return v, nil
}

// SystemVersion returns version information about the daemon.
//
// The only error this should return is due to context cancellation/deadline.
// Anything else should be logged and ignored because this is looking up
// multiple things and is often used for debugging.
// The only case valid early return is when the caller doesn't want the result anymore (ie context cancelled).
func (daemon *Daemon) SystemVersion(ctx context.Context) (system.VersionResponse, error) {
	defer metrics.StartTimer(metrics.HostInfoFunctions.WithValues("system_version"))()

	kernelVer := kernelVersion(ctx)
	cfg := daemon.config()

	v := system.VersionResponse{
		Components: []system.ComponentVersion{
			{
				Name:    "Engine",
				Version: dockerversion.Version,
				Details: map[string]string{
					"GitCommit":     dockerversion.GitCommit,
					"ApiVersion":    config.MaxAPIVersion,
					"MinAPIVersion": cfg.MinAPIVersion,
					"GoVersion":     runtime.Version(),
					"Os":            runtime.GOOS,
					"Arch":          runtime.GOARCH,
					"BuildTime":     dockerversion.BuildTime,
					"KernelVersion": kernelVer,
					"Experimental":  strconv.FormatBool(cfg.Experimental),
				},
			},
		},

		// Populate deprecated fields for older clients
		Version:       dockerversion.Version,
		GitCommit:     dockerversion.GitCommit,
		APIVersion:    config.MaxAPIVersion,
		MinAPIVersion: cfg.MinAPIVersion,
		GoVersion:     runtime.Version(),
		Os:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		BuildTime:     dockerversion.BuildTime,
		KernelVersion: kernelVer,
		Experimental:  cfg.Experimental,
	}

	v.Platform.Name = dockerversion.PlatformName

	if err := daemon.fillPlatformVersion(ctx, &v, cfg); err != nil {
		return v, err
	}
	return v, nil
}

func (daemon *Daemon) fillDriverInfo(v *system.Info) {
	v.Driver = daemon.imageService.StorageDriver()
	v.DriverStatus = daemon.imageService.LayerStoreStatus()

	const warnMsg = `
WARNING: The %s storage-driver is deprecated, and will be removed in a future release.
         Refer to the documentation for more information: https://docs.docker.com/go/storage-driver/`

	switch v.Driver {
	case "overlay":
		v.Warnings = append(v.Warnings, fmt.Sprintf(warnMsg, v.Driver))
	}

	fillDriverWarnings(v)
}

func (daemon *Daemon) fillPluginsInfo(ctx context.Context, v *system.Info, cfg *config.Config) {
	v.Plugins = system.PluginsInfo{
		Volume:  daemon.volumes.GetDriverList(),
		Network: daemon.GetNetworkDriverList(ctx),

		// The authorization plugins are returned in the order they are
		// used as they constitute a request/response modification chain.
		Authorization: cfg.AuthorizationPlugins,
		Log:           logger.ListDrivers(),
	}
}

// fillSecurityOptions fills the [system.Info.SecurityOptions] field based
// on the daemon configuration.
//
// TODO(thaJeztah): consider making [system.Info.SecurityOptions] a structured response as originally intended in https://github.com/moby/moby/pull/26276
func (daemon *Daemon) fillSecurityOptions(v *system.Info, sysInfo *sysinfo.SysInfo, cfg *config.Config) {
	var securityOptions []string
	if sysInfo.AppArmor {
		securityOptions = append(securityOptions, "name=apparmor")
	}
	if sysInfo.Seccomp && supportsSeccomp {
		if daemon.seccompProfilePath != config.SeccompProfileDefault {
			v.Warnings = append(v.Warnings, "WARNING: daemon is not using the default seccomp profile")
		}
		securityOptions = append(securityOptions, "name=seccomp,profile="+daemon.seccompProfilePath)
	}
	if selinux.GetEnabled() {
		securityOptions = append(securityOptions, "name=selinux")
	}
	if uid, gid := daemon.idMapping.RootPair(); uid != 0 || gid != 0 {
		securityOptions = append(securityOptions, "name=userns")
	}
	if Rootless(cfg) {
		securityOptions = append(securityOptions, "name=rootless")
	}
	if cgroupNamespacesEnabled(sysInfo, cfg) {
		securityOptions = append(securityOptions, "name=cgroupns")
	}
	if noNewPrivileges(cfg) {
		securityOptions = append(securityOptions, "name=no-new-privileges")
	}

	v.SecurityOptions = securityOptions
}

func (daemon *Daemon) fillContainerStates(v *system.Info) {
	cRunning, cPaused, cStopped := metrics.StateCtr.Get()
	v.Containers = cRunning + cPaused + cStopped
	v.ContainersPaused = cPaused
	v.ContainersRunning = cRunning
	v.ContainersStopped = cStopped
}

// fillDebugInfo sets the current debugging state of the daemon, and additional
// debugging information, such as the number of Go-routines, and file descriptors.
//
// Note that this currently always collects the information, but the CLI only
// prints it if the daemon has debug enabled. We should consider to either make
// this information optional (cli to request "with debugging information"), or
// only collect it if the daemon has debug enabled. For the CLI code, see
// https://github.com/docker/cli/blob/v20.10.12/cli/command/system/info.go#L239-L244
func (daemon *Daemon) fillDebugInfo(ctx context.Context, v *system.Info) {
	v.Debug = debug.IsEnabled()
	v.NFd = filedescriptors.GetTotalUsedFds(ctx)
	v.NGoroutines = runtime.NumGoroutine()
	v.NEventsListener = daemon.EventsService.SubscribersCount()
}

// fillContainerdInfo provides information about the containerd configuration
// for debugging purposes.
func (daemon *Daemon) fillContainerdInfo(v *system.Info, cfg *config.Config) {
	if cfg.ContainerdAddr == "" {
		return
	}
	v.Containerd = &system.ContainerdInfo{
		Address: cfg.ContainerdAddr,
		Namespaces: system.ContainerdNamespaces{
			Containers: cfg.ContainerdNamespace,
			Plugins:    cfg.ContainerdPluginNamespace,
		},
	}
}

func (daemon *Daemon) fillAPIInfo(v *system.Info, cfg *config.Config) {
	const warn string = `
         Access to the remote API is equivalent to root access on the host. Refer
         to the 'Docker daemon attack surface' section in the documentation for
         more information: https://docs.docker.com/go/attack-surface/`

	for _, host := range cfg.Hosts {
		// cnf.Hosts is normalized during startup, so should always have a scheme/proto
		proto, addr, _ := strings.Cut(host, "://")
		if proto != "tcp" {
			continue
		}
		const removal = "In future versions this will be a hard failure preventing the daemon from starting! Learn more at: https://docs.docker.com/go/api-security/"
		if cfg.TLS == nil || !*cfg.TLS {
			v.Warnings = append(v.Warnings, fmt.Sprintf("[DEPRECATION NOTICE]: API is accessible on http://%s without encryption.%s\n%s", addr, warn, removal))
			continue
		}
		if cfg.TLSVerify == nil || !*cfg.TLSVerify {
			v.Warnings = append(v.Warnings, fmt.Sprintf("[DEPRECATION NOTICE]: API is accessible on https://%s without TLS client verification.%s\n%s", addr, warn, removal))
			continue
		}
	}
}

func (daemon *Daemon) fillDefaultAddressPools(ctx context.Context, v *system.Info, cfg *config.Config) {
	_, span := tracing.StartSpan(ctx, "fillDefaultAddressPools")
	defer span.End()
	for _, pool := range cfg.DefaultAddressPools.Value() {
		v.DefaultAddressPools = append(v.DefaultAddressPools, system.NetworkAddressPool{
			Base: pool.Base,
			Size: pool.Size,
		})
	}
}

func (daemon *Daemon) fillFirewallInfo(v *system.Info) {
	if daemon.netController == nil {
		return
	}
	v.FirewallBackend = daemon.netController.FirewallBackend()
}

func hostName(ctx context.Context) string {
	ctx, span := tracing.StartSpan(ctx, "hostName")
	defer span.End()
	hn, err := os.Hostname()
	if err != nil {
		log.G(ctx).WithError(err).Warn("Could not get hostname")
		return ""
	}
	return hn
}

func kernelVersion(ctx context.Context) string {
	ctx, span := tracing.StartSpan(ctx, "kernelVersion")
	defer span.End()

	var ver string
	if kv, err := kernel.GetKernelVersion(); err != nil {
		log.G(ctx).WithError(err).Warn("Could not get kernel version")
	} else {
		ver = kv.String()
	}
	return ver
}

func memInfo(ctx context.Context) *meminfo.Memory {
	ctx, span := tracing.StartSpan(ctx, "memInfo")
	defer span.End()

	mi, err := meminfo.Read()
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not read system memory info")
		return &meminfo.Memory{}
	}
	return mi
}

func operatingSystem(ctx context.Context) (operatingSystem string) {
	ctx, span := tracing.StartSpan(ctx, "operatingSystem")
	defer span.End()

	defer metrics.StartTimer(metrics.HostInfoFunctions.WithValues("operating_system"))()

	if s, err := operatingsystem.GetOperatingSystem(); err != nil {
		log.G(ctx).WithError(err).Warn("Could not get operating system name")
	} else {
		operatingSystem = s
	}
	if inContainer, err := operatingsystem.IsContainerized(); err != nil {
		log.G(ctx).WithError(err).Error("Could not determine if daemon is containerized")
		operatingSystem += " (error determining if containerized)"
	} else if inContainer {
		operatingSystem += " (containerized)"
	}

	return operatingSystem
}

func osVersion(ctx context.Context) (version string) {
	ctx, span := tracing.StartSpan(ctx, "osVersion")
	defer span.End()

	defer metrics.StartTimer(metrics.HostInfoFunctions.WithValues("os_version"))()

	version, err := operatingsystem.GetOperatingSystemVersion()
	if err != nil {
		log.G(ctx).WithError(err).Warn("Could not get operating system version")
		return ""
	}

	return version
}

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val := os.Getenv(n); val != "" {
			return val
		}
	}
	return ""
}

func getConfigOrEnv(config string, env ...string) string {
	if config != "" {
		return config
	}
	return getEnvAny(env...)
}

// promoteNil converts a nil slice to an empty slice of that type.
// A non-nil slice is returned as is.
func promoteNil[S ~[]E, E any](s S) S {
	if s == nil {
		return S{}
	}
	return s
}

// fillDiscoveredDevicesFromDrivers iterates over registered device drivers
// and calls their ListDevices method (if available) to populate system info.
func (daemon *Daemon) fillDiscoveredDevicesFromDrivers(ctx context.Context, v *system.Info, cfg *config.Config) {
	ctx, span := tracing.StartSpan(ctx, "daemon.fillDiscoveredDevicesFromDrivers")
	defer span.End()

	// Make sure v.DiscoveredDevices is initialized to an empty slice instead of nil.
	// This ensures that the JSON output is always a valid array, even if no devices are discovered.
	v.DiscoveredDevices = []system.DeviceInfo{}

	for driverName, driver := range deviceDrivers {
		if driver.ListDevices == nil {
			log.G(ctx).WithField("driver", driverName).Trace("Device driver does not implement ListDevices method.")
			continue
		}

		ls, err := driver.ListDevices(ctx, cfg)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"driver": driverName,
				"error":  err,
			}).Warn("Failed to list devices for driver")
			v.Warnings = append(v.Warnings, fmt.Sprintf("Failed to list devices from driver '%s': %v", driverName, err))
			continue
		}

		if len(ls.Warnings) > 0 {
			v.Warnings = append(v.Warnings, ls.Warnings...)
		}

		for _, device := range ls.Devices {
			device.Source = driverName
			v.DiscoveredDevices = append(v.DiscoveredDevices, device)
		}
	}
}
