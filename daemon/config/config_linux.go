package config // import "github.com/docker/docker/daemon/config"

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/pkg/rootless"
	"github.com/pkg/errors"
)

const (
	// DefaultIpcMode is default for container's IpcMode, if not set otherwise
	DefaultIpcMode = container.IPCModePrivate

	// DefaultCgroupNamespaceMode is the default mode for containers cgroup namespace when using cgroups v2.
	DefaultCgroupNamespaceMode = container.CgroupnsModePrivate

	// DefaultCgroupV1NamespaceMode is the default mode for containers cgroup namespace when using cgroups v1.
	DefaultCgroupV1NamespaceMode = container.CgroupnsModeHost

	// StockRuntimeName is the reserved name/alias used to represent the
	// OCI runtime being shipped with the docker daemon package.
	StockRuntimeName = "runc"

	// userlandProxyBinary is the name of the userland-proxy binary.
	// In rootless-mode, [rootless.RootlessKitDockerProxyBinary] is used instead.
	userlandProxyBinary = "docker-proxy"
)

// BridgeConfig stores all the parameters for both the bridge driver and the default bridge network.
type BridgeConfig struct {
	DefaultBridgeConfig

	EnableIPTables      bool   `json:"iptables,omitempty"`
	EnableIP6Tables     bool   `json:"ip6tables,omitempty"`
	EnableIPForward     bool   `json:"ip-forward,omitempty"`
	EnableIPMasq        bool   `json:"ip-masq,omitempty"`
	EnableUserlandProxy bool   `json:"userland-proxy,omitempty"`
	UserlandProxyPath   string `json:"userland-proxy-path,omitempty"`
}

// DefaultBridgeConfig stores all the parameters for the default bridge network.
type DefaultBridgeConfig struct {
	commonBridgeConfig

	// Fields below here are platform specific.
	EnableIPv6                  bool   `json:"ipv6,omitempty"`
	FixedCIDRv6                 string `json:"fixed-cidr-v6,omitempty"`
	MTU                         int    `json:"mtu,omitempty"`
	DefaultIP                   net.IP `json:"ip,omitempty"`
	IP                          string `json:"bip,omitempty"`
	DefaultGatewayIPv4          net.IP `json:"default-gateway,omitempty"`
	DefaultGatewayIPv6          net.IP `json:"default-gateway-v6,omitempty"`
	InterContainerCommunication bool   `json:"icc,omitempty"`
}

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// Fields below here are platform specific.
	Runtimes             map[string]system.Runtime    `json:"runtimes,omitempty"`
	DefaultInitBinary    string                       `json:"default-init,omitempty"`
	CgroupParent         string                       `json:"cgroup-parent,omitempty"`
	EnableSelinuxSupport bool                         `json:"selinux-enabled,omitempty"`
	RemappedRoot         string                       `json:"userns-remap,omitempty"`
	Ulimits              map[string]*container.Ulimit `json:"default-ulimits,omitempty"`
	CPURealtimePeriod    int64                        `json:"cpu-rt-period,omitempty"`
	CPURealtimeRuntime   int64                        `json:"cpu-rt-runtime,omitempty"`
	Init                 bool                         `json:"init,omitempty"`
	InitPath             string                       `json:"init-path,omitempty"`
	SeccompProfile       string                       `json:"seccomp-profile,omitempty"`
	ShmSize              opts.MemBytes                `json:"default-shm-size,omitempty"`
	NoNewPrivileges      bool                         `json:"no-new-privileges,omitempty"`
	IpcMode              string                       `json:"default-ipc-mode,omitempty"`
	CgroupNamespaceMode  string                       `json:"default-cgroupns-mode,omitempty"`
	// ResolvConf is the path to the configuration of the host resolver
	ResolvConf string `json:"resolv-conf,omitempty"`
	Rootless   bool   `json:"rootless,omitempty"`
}

// GetExecRoot returns the user configured Exec-root
func (conf *Config) GetExecRoot() string {
	return conf.ExecRoot
}

// GetInitPath returns the configured docker-init path
func (conf *Config) GetInitPath() string {
	if conf.InitPath != "" {
		return conf.InitPath
	}
	if conf.DefaultInitBinary != "" {
		return conf.DefaultInitBinary
	}
	return DefaultInitBinary
}

// lookupBinPath returns an absolute path to the provided binary by searching relevant "libexec" locations (per FHS 3.0 & 2.3) followed by PATH
func lookupBinPath(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		return binary, nil
	}

	lookupPaths := []string{
		// FHS 3.0: "/usr/libexec includes internal binaries that are not intended to be executed directly by users or shell scripts. Applications may use a single subdirectory under /usr/libexec."
		// https://refspecs.linuxfoundation.org/FHS_3.0/fhs/ch04s07.html
		"/usr/local/libexec/docker",
		"/usr/libexec/docker",

		// FHS 2.3: "/usr/lib includes object files, libraries, and internal binaries that are not intended to be executed directly by users or shell scripts."
		// https://refspecs.linuxfoundation.org/FHS_2.3/fhs-2.3.html#USRLIBLIBRARIESFORPROGRAMMINGANDPA
		"/usr/local/lib/docker",
		"/usr/lib/docker",
	}

	// According to FHS 3.0, it is not necessary to have a subdir here (see note and reference above).
	// If the binary has a `docker-` prefix, let's look it up without the dir prefix.
	if strings.HasPrefix(binary, "docker-") {
		lookupPaths = append(lookupPaths, "/usr/local/libexec")
		lookupPaths = append(lookupPaths, "/usr/libexec")
	}

	for _, dir := range lookupPaths {
		// exec.LookPath has a fast-path short-circuit for paths that contain "/" (skipping the PATH lookup) that then verifies whether the given path is likely to be an actual executable binary (so we invoke that instead of reimplementing the same checks)
		if file, err := exec.LookPath(filepath.Join(dir, binary)); err == nil {
			return file, nil
		}
	}

	// if we checked all the "libexec" directories and found no matches, fall back to PATH
	return exec.LookPath(binary)
}

// LookupInitPath returns an absolute path to the "docker-init" binary by searching relevant "libexec" directories (per FHS 3.0 & 2.3) followed by PATH
func (conf *Config) LookupInitPath() (string, error) {
	return lookupBinPath(conf.GetInitPath())
}

// GetResolvConf returns the appropriate resolv.conf
// Check setupResolvConf on how this is selected
func (conf *Config) GetResolvConf() string {
	return conf.ResolvConf
}

// IsSwarmCompatible defines if swarm mode can be enabled in this config
func (conf *Config) IsSwarmCompatible() error {
	if conf.LiveRestoreEnabled {
		return fmt.Errorf("--live-restore daemon configuration is incompatible with swarm mode")
	}
	return nil
}

func verifyDefaultIpcMode(mode string) error {
	const hint = `use "shareable" or "private"`

	dm := container.IpcMode(mode)
	if !dm.Valid() {
		return fmt.Errorf("default IPC mode setting (%v) is invalid; "+hint, dm)
	}
	if dm != "" && !dm.IsPrivate() && !dm.IsShareable() {
		return fmt.Errorf(`IPC mode "%v" is not supported as default value; `+hint, dm)
	}
	return nil
}

func verifyDefaultCgroupNsMode(mode string) error {
	cm := container.CgroupnsMode(mode)
	if !cm.Valid() {
		return fmt.Errorf(`default cgroup namespace mode (%v) is invalid; use "host" or "private"`, cm)
	}

	return nil
}

// ValidatePlatformConfig checks if any platform-specific configuration settings are invalid.
func (conf *Config) ValidatePlatformConfig() error {
	if conf.EnableUserlandProxy {
		if conf.UserlandProxyPath == "" {
			return errors.New("invalid userland-proxy-path: userland-proxy is enabled, but userland-proxy-path is not set")
		}
		if !filepath.IsAbs(conf.UserlandProxyPath) {
			return errors.New("invalid userland-proxy-path: must be an absolute path: " + conf.UserlandProxyPath)
		}
		// Using exec.LookPath here, because it also produces an error if the
		// given path is not a valid executable or a directory.
		if _, err := exec.LookPath(conf.UserlandProxyPath); err != nil {
			return errors.Wrap(err, "invalid userland-proxy-path")
		}
	}

	if err := verifyDefaultIpcMode(conf.IpcMode); err != nil {
		return err
	}

	if err := bridge.ValidateFixedCIDRV6(conf.FixedCIDRv6); err != nil {
		return errors.Wrap(err, "invalid fixed-cidr-v6")
	}

	if _, ok := conf.Features["windows-dns-proxy"]; ok {
		return errors.New("feature option 'windows-dns-proxy' is only available on Windows")
	}

	return verifyDefaultCgroupNsMode(conf.CgroupNamespaceMode)
}

// IsRootless returns conf.Rootless on Linux but false on Windows
func (conf *Config) IsRootless() bool {
	return conf.Rootless
}

func setPlatformDefaults(cfg *Config) error {
	cfg.Ulimits = make(map[string]*container.Ulimit)
	cfg.ShmSize = opts.MemBytes(DefaultShmSize)
	cfg.SeccompProfile = SeccompProfileDefault
	cfg.IpcMode = string(DefaultIpcMode)
	cfg.Runtimes = make(map[string]system.Runtime)

	if cgroups.Mode() != cgroups.Unified {
		cfg.CgroupNamespaceMode = string(DefaultCgroupV1NamespaceMode)
	} else {
		cfg.CgroupNamespaceMode = string(DefaultCgroupNamespaceMode)
	}

	if rootless.RunningWithRootlessKit() {
		cfg.Rootless = true

		var err error
		// use rootlesskit-docker-proxy for exposing the ports in RootlessKit netns to the initial namespace.
		cfg.BridgeConfig.UserlandProxyPath, err = lookupBinPath(rootless.RootlessKitDockerProxyBinary)
		if err != nil {
			return errors.Wrapf(err, "running with RootlessKit, but %s not installed", rootless.RootlessKitDockerProxyBinary)
		}

		dataHome, err := homedir.GetDataHome()
		if err != nil {
			return err
		}
		runtimeDir, err := homedir.GetRuntimeDir()
		if err != nil {
			return err
		}

		cfg.Root = filepath.Join(dataHome, "docker")
		cfg.ExecRoot = filepath.Join(runtimeDir, "docker")
		cfg.Pidfile = filepath.Join(runtimeDir, "docker.pid")
	} else {
		var err error
		cfg.BridgeConfig.UserlandProxyPath, err = lookupBinPath(userlandProxyBinary)
		if err != nil {
			// Log, but don't error here. This allows running a daemon with
			// userland-proxy disabled (which does not require the binary
			// to be present).
			//
			// An error is still produced by [Config.ValidatePlatformConfig] if
			// userland-proxy is enabled in the configuration.
			//
			// We log this at "debug" level, as this code is also executed
			// when running "--version", and we don't want to print logs in
			// that case..
			log.G(context.TODO()).WithError(err).Debug("failed to lookup default userland-proxy binary")
		}
		cfg.Root = "/var/lib/docker"
		cfg.ExecRoot = "/var/run/docker"
		cfg.Pidfile = "/var/run/docker.pid"
	}

	return nil
}
