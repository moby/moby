package config // import "github.com/docker/docker/daemon/config"

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"

	"github.com/containerd/cgroups"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/rootless"
	units "github.com/docker/go-units"
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
)

// BridgeConfig stores all the bridge driver specific
// configuration.
type BridgeConfig struct {
	commonBridgeConfig

	// Fields below here are platform specific.
	DefaultIP                   net.IP `json:"ip,omitempty"`
	IP                          string `json:"bip,omitempty"`
	DefaultGatewayIPv4          net.IP `json:"default-gateway,omitempty"`
	DefaultGatewayIPv6          net.IP `json:"default-gateway-v6,omitempty"`
	InterContainerCommunication bool   `json:"icc,omitempty"`

	EnableIPv6          bool   `json:"ipv6,omitempty"`
	EnableIPTables      bool   `json:"iptables,omitempty"`
	EnableIP6Tables     bool   `json:"ip6tables,omitempty"`
	EnableIPForward     bool   `json:"ip-forward,omitempty"`
	EnableIPMasq        bool   `json:"ip-masq,omitempty"`
	EnableUserlandProxy bool   `json:"userland-proxy,omitempty"`
	UserlandProxyPath   string `json:"userland-proxy-path,omitempty"`
	FixedCIDRv6         string `json:"fixed-cidr-v6,omitempty"`
}

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// Fields below here are platform specific.
	Runtimes             map[string]types.Runtime `json:"runtimes,omitempty"`
	DefaultInitBinary    string                   `json:"default-init,omitempty"`
	CgroupParent         string                   `json:"cgroup-parent,omitempty"`
	EnableSelinuxSupport bool                     `json:"selinux-enabled,omitempty"`
	RemappedRoot         string                   `json:"userns-remap,omitempty"`
	Ulimits              map[string]*units.Ulimit `json:"default-ulimits,omitempty"`
	CPURealtimePeriod    int64                    `json:"cpu-rt-period,omitempty"`
	CPURealtimeRuntime   int64                    `json:"cpu-rt-runtime,omitempty"`
	OOMScoreAdjust       int                      `json:"oom-score-adjust,omitempty"`
	Init                 bool                     `json:"init,omitempty"`
	InitPath             string                   `json:"init-path,omitempty"`
	SeccompProfile       string                   `json:"seccomp-profile,omitempty"`
	ShmSize              opts.MemBytes            `json:"default-shm-size,omitempty"`
	NoNewPrivileges      bool                     `json:"no-new-privileges,omitempty"`
	IpcMode              string                   `json:"default-ipc-mode,omitempty"`
	CgroupNamespaceMode  string                   `json:"default-cgroupns-mode,omitempty"`
	// ResolvConf is the path to the configuration of the host resolver
	ResolvConf string `json:"resolv-conf,omitempty"`
	Rootless   bool   `json:"rootless,omitempty"`
}

// GetRuntime returns the runtime path and arguments for a given
// runtime name
func (conf *Config) GetRuntime(name string) *types.Runtime {
	conf.Lock()
	defer conf.Unlock()
	if rt, ok := conf.Runtimes[name]; ok {
		return &rt
	}
	return nil
}

// GetAllRuntimes returns a copy of the runtimes map
func (conf *Config) GetAllRuntimes() map[string]types.Runtime {
	conf.Lock()
	rts := conf.Runtimes
	conf.Unlock()
	return rts
}

// GetExecRoot returns the user configured Exec-root
func (conf *Config) GetExecRoot() string {
	return conf.ExecRoot
}

// GetInitPath returns the configured docker-init path
func (conf *Config) GetInitPath() string {
	conf.Lock()
	defer conf.Unlock()
	if conf.InitPath != "" {
		return conf.InitPath
	}
	if conf.DefaultInitBinary != "" {
		return conf.DefaultInitBinary
	}
	return DefaultInitBinary
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
	if err := verifyDefaultIpcMode(conf.IpcMode); err != nil {
		return err
	}

	return verifyDefaultCgroupNsMode(conf.CgroupNamespaceMode)
}

// IsRootless returns conf.Rootless on Linux but false on Windows
func (conf *Config) IsRootless() bool {
	return conf.Rootless
}

func setPlatformDefaults(cfg *Config) error {
	cfg.Ulimits = make(map[string]*units.Ulimit)
	cfg.ShmSize = opts.MemBytes(DefaultShmSize)
	cfg.SeccompProfile = SeccompProfileDefault
	cfg.IpcMode = string(DefaultIpcMode)
	cfg.Runtimes = make(map[string]types.Runtime)

	if cgroups.Mode() != cgroups.Unified {
		cfg.CgroupNamespaceMode = string(DefaultCgroupV1NamespaceMode)
	} else {
		cfg.CgroupNamespaceMode = string(DefaultCgroupNamespaceMode)
	}

	if rootless.RunningWithRootlessKit() {
		cfg.Rootless = true

		var err error
		// use rootlesskit-docker-proxy for exposing the ports in RootlessKit netns to the initial namespace.
		cfg.BridgeConfig.UserlandProxyPath, err = exec.LookPath(rootless.RootlessKitDockerProxyBinary)
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
		cfg.Root = "/var/lib/docker"
		cfg.ExecRoot = "/var/run/docker"
		cfg.Pidfile = "/var/run/docker.pid"
	}

	return nil
}
