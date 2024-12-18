// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd/tracing"
	"github.com/containerd/log"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/cmd/dockerd/debug"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/internal/platform"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/meminfo"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/parsers/operatingsystem"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/registry"
	"github.com/docker/go-metrics"
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
	defer metrics.StartTimer(hostInfoFunctions.WithValues("system_info"))()

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
		NCPU:               doWithTrace(ctx, "sysinfo.NumCPU", sysinfo.NumCPU),
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

	return v, nil
}

// SystemVersion returns version information about the daemon.
//
// The only error this should return is due to context cancellation/deadline.
// Anything else should be logged and ignored because this is looking up
// multiple things and is often used for debugging.
// The only case valid early return is when the caller doesn't want the result anymore (ie context cancelled).
func (daemon *Daemon) SystemVersion(ctx context.Context) (types.Version, error) {
	defer metrics.StartTimer(hostInfoFunctions.WithValues("system_version"))()

	kernelVersion := kernelVersion(ctx)
	cfg := daemon.config()

	v := types.Version{
		Components: []types.ComponentVersion{
			{
				Name:    "Engine",
				Version: dockerversion.Version,
				Details: map[string]string{
					"GitCommit":     dockerversion.GitCommit,
					"ApiVersion":    api.DefaultVersion,
					"MinAPIVersion": cfg.MinAPIVersion,
					"GoVersion":     runtime.Version(),
					"Os":            runtime.GOOS,
					"Arch":          runtime.GOARCH,
					"BuildTime":     dockerversion.BuildTime,
					"KernelVersion": kernelVersion,
					"Experimental":  fmt.Sprintf("%t", cfg.Experimental),
				},
			},
		},

		// Populate deprecated fields for older clients
		Version:       dockerversion.Version,
		GitCommit:     dockerversion.GitCommit,
		APIVersion:    api.DefaultVersion,
		MinAPIVersion: cfg.MinAPIVersion,
		GoVersion:     runtime.Version(),
		Os:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		BuildTime:     dockerversion.BuildTime,
		KernelVersion: kernelVersion,
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
	if rootIDs := daemon.idMapping.RootPair(); rootIDs.UID != 0 || rootIDs.GID != 0 {
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
	cRunning, cPaused, cStopped := stateCtr.get()
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
	v.NFd = fileutils.GetTotalUsedFds(ctx)
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

	if cfg.CorsHeaders != "" {
		v.Warnings = append(v.Warnings, `DEPRECATED: The "api-cors-header" config parameter and the dockerd "--api-cors-header" option will be removed in the next release. Use a reverse proxy if you need CORS headers.`)
	}

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
			Base: pool.Base.String(),
			Size: pool.Size,
		})
	}
}

func hostName(ctx context.Context) string {
	ctx, span := tracing.StartSpan(ctx, "hostName")
	defer span.End()
	hostname := ""
	if hn, err := os.Hostname(); err != nil {
		log.G(ctx).Warnf("Could not get hostname: %v", err)
	} else {
		hostname = hn
	}
	return hostname
}

func kernelVersion(ctx context.Context) string {
	ctx, span := tracing.StartSpan(ctx, "kernelVersion")
	defer span.End()

	var kernelVersion string
	if kv, err := kernel.GetKernelVersion(); err != nil {
		log.G(ctx).Warnf("Could not get kernel version: %v", err)
	} else {
		kernelVersion = kv.String()
	}
	return kernelVersion
}

func memInfo(ctx context.Context) *meminfo.Memory {
	ctx, span := tracing.StartSpan(ctx, "memInfo")
	defer span.End()

	memInfo, err := meminfo.Read()
	if err != nil {
		log.G(ctx).Errorf("Could not read system memory info: %v", err)
		memInfo = &meminfo.Memory{}
	}
	return memInfo
}

func operatingSystem(ctx context.Context) (operatingSystem string) {
	ctx, span := tracing.StartSpan(ctx, "operatingSystem")
	defer span.End()

	defer metrics.StartTimer(hostInfoFunctions.WithValues("operating_system"))()

	if s, err := operatingsystem.GetOperatingSystem(); err != nil {
		log.G(ctx).Warnf("Could not get operating system name: %v", err)
	} else {
		operatingSystem = s
	}
	if inContainer, err := operatingsystem.IsContainerized(); err != nil {
		log.G(ctx).Errorf("Could not determine if daemon is containerized: %v", err)
		operatingSystem += " (error determining if containerized)"
	} else if inContainer {
		operatingSystem += " (containerized)"
	}

	return operatingSystem
}

func osVersion(ctx context.Context) (version string) {
	ctx, span := tracing.StartSpan(ctx, "osVersion")
	defer span.End()

	defer metrics.StartTimer(hostInfoFunctions.WithValues("os_version"))()

	version, err := operatingsystem.GetOperatingSystemVersion()
	if err != nil {
		log.G(ctx).Warnf("Could not get operating system version: %v", err)
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

// promoteNil converts a nil slice to an empty slice.
// A non-nil slice is returned as is.
//
// TODO: make generic again once we are a go module,
// go.dev/issue/64759 is fixed, or we drop support for Go 1.21.
func promoteNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
