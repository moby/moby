//go:build !linux && !windows

package config

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/pkg/opts"
)

// StockRuntimeName is the name of the default OCI runtime. Referenced from
// shared Unix code (daemon/runtime_unix.go, daemon/command/config_unix.go)
// even on platforms where no runtime can actually be invoked.
const StockRuntimeName = "runc"

// BridgeConfig keeps the linux JSON / CLI shape so the daemon config
// package compiles on unsupported platforms. None of the iptables /
// ip-forward / userland-proxy fields are meaningful at runtime here;
// they exist only so cross-platform CLI / JSON wiring continues to
// type-check.
type BridgeConfig struct {
	DefaultBridgeConfig

	EnableIPTables           bool   `json:"iptables,omitempty"`
	EnableIP6Tables          bool   `json:"ip6tables,omitempty"`
	EnableIPForward          bool   `json:"ip-forward,omitempty"`
	DisableFilterForwardDrop bool   `json:"ip-forward-no-drop,omitempty"`
	EnableIPMasq             bool   `json:"ip-masq,omitempty"`
	EnableUserlandProxy      bool   `json:"userland-proxy,omitempty"`
	UserlandProxyPath        string `json:"userland-proxy-path,omitempty"`
	AllowDirectRouting       bool   `json:"allow-direct-routing,omitempty"`
	BridgeAcceptFwMark       string `json:"bridge-accept-fwmark,omitempty"`
}

// DefaultBridgeConfig keeps the linux JSON / CLI shape, see BridgeConfig.
type DefaultBridgeConfig struct {
	commonBridgeConfig

	EnableIPv6                  bool   `json:"ipv6,omitempty"`
	FixedCIDRv6                 string `json:"fixed-cidr-v6,omitempty"`
	MTU                         int    `json:"mtu,omitempty"`
	DefaultIP                   net.IP `json:"ip,omitempty"`
	IP                          string `json:"bip,omitempty"`
	IP6                         string `json:"bip6,omitempty"`
	DefaultGatewayIPv4          net.IP `json:"default-gateway,omitempty"`
	DefaultGatewayIPv6          net.IP `json:"default-gateway-v6,omitempty"`
	InterContainerCommunication bool   `json:"icc,omitempty"`
}

// Config keeps the linux JSON / CLI shape. Most fields are unused at
// runtime on unsupported platforms but the daemon's cross-platform CLI
// wiring (`daemon/command/config_unix.go`) needs them to exist.
type Config struct {
	CommonConfig

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
	ResolvConf           string                       `json:"resolv-conf,omitempty"`
	Rootless             bool                         `json:"rootless,omitempty"`
}

func (conf *Config) GetExecRoot() string { return conf.ExecRoot }

func (conf *Config) GetInitPath() string {
	if conf.InitPath != "" {
		return conf.InitPath
	}
	if conf.DefaultInitBinary != "" {
		return conf.DefaultInitBinary
	}
	return DefaultInitBinary
}

func (conf *Config) LookupInitPath() (string, error) { return lookupBinPath(conf.GetInitPath()) }

func (conf *Config) GetResolvConf() string { return conf.ResolvConf }

func (conf *Config) IsSwarmCompatible() error {
	if conf.LiveRestoreEnabled {
		return errors.New("--live-restore daemon configuration is incompatible with swarm mode")
	}
	return nil
}

func (conf *Config) IsRootless() bool { return conf.Rootless }

func setPlatformDefaults(cfg *Config) error {
	cfg.Ulimits = make(map[string]*container.Ulimit)
	cfg.Runtimes = make(map[string]system.Runtime)
	cfg.ShmSize = opts.MemBytes(DefaultShmSize)
	cfg.SeccompProfile = SeccompProfileDefault
	return nil
}

func validatePlatformConfig(*Config) error { return nil }

func validatePlatformExecOpt(opt, _ string) error {
	return fmt.Errorf("exec opt %q is not supported on this platform", opt)
}

// lookupBinPath looks up binary in $PATH. The /usr/libexec/docker FHS
// search done on linux is intentionally omitted: the daemon does not run
// on unsupported platforms, so the FHS layout is not assumed.
func lookupBinPath(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		return binary, nil
	}
	return exec.LookPath(binary)
}
