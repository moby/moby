// +build linux freebsd

package config

import (
	"fmt"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/opts"
	units "github.com/docker/go-units"
)

const (
	// DefaultIpcMode is default for container's IpcMode, if not set otherwise
	DefaultIpcMode = "shareable" // TODO: change to private
)

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// These fields are common to all unix platforms.
	CommonUnixConfig

	// Fields below here are platform specific.
	CgroupParent         string                   `json:"cgroup-parent,omitempty" toml:"cgroup-parent,omitempty"`
	EnableSelinuxSupport bool                     `json:"selinux-enabled,omitempty" toml:"selinux-enabled,omitempty"`
	RemappedRoot         string                   `json:"userns-remap,omitempty" toml:"userns-remap,omitempty"`
	Ulimits              map[string]*units.Ulimit `json:"default-ulimits,omitempty" toml:"default-ulimits,omitempty"`
	CPURealtimePeriod    int64                    `json:"cpu-rt-period,omitempty" toml:"cpu-rt-period,omitempty"`
	CPURealtimeRuntime   int64                    `json:"cpu-rt-runtime,omitempty" toml:"cpu-rt-runtime,omitempty"`
	OOMScoreAdjust       int                      `json:"oom-score-adjust,omitempty" toml:"oom-score-adjust,omitempty"`
	Init                 bool                     `json:"init,omitempty" toml:"init,omitempty"`
	InitPath             string                   `json:"init-path,omitempty" toml:"init-path,omitempty"`
	SeccompProfile       string                   `json:"seccomp-profile,omitempty" toml:"seccomp-profile,omitempty"`
	ShmSize              opts.MemBytes            `json:"default-shm-size,omitempty" toml:"default-shm-size,omitempty"`
	NoNewPrivileges      bool                     `json:"no-new-privileges,omitempty" toml:"no-new-privileges,omitempty"`
	IpcMode              string                   `json:"default-ipc-mode,omitempty" toml:"default-ipc-mode,omitempty"`
}

// BridgeConfig stores all the bridge driver specific
// configuration.
type BridgeConfig struct {
	commonBridgeConfig

	// These fields are common to all unix platforms.
	commonUnixBridgeConfig

	// Fields below here are platform specific.
	EnableIPv6          bool   `json:"ipv6,omitempty" toml:"ipv6,omitempty"`
	EnableIPTables      bool   `json:"iptables,omitempty" toml:"iptables,omitempty"`
	EnableIPForward     bool   `json:"ip-forward,omitempty" toml:"ip-forward,omitempty"`
	EnableIPMasq        bool   `json:"ip-masq,omitempty" toml:"ip-masq,omitempty"`
	EnableUserlandProxy bool   `json:"userland-proxy,omitempty" toml:"userland-proxy,omitempty"`
	UserlandProxyPath   string `json:"userland-proxy-path,omitempty" toml:"userland-proxy-path,omitempty"`
	FixedCIDRv6         string `json:"fixed-cidr-v6,omitempty" toml:"fixed-cidr-v6,omitempty"`
}

// IsSwarmCompatible defines if swarm mode can be enabled in this config
func (conf *Config) IsSwarmCompatible() error {
	if conf.ClusterStore != "" || conf.ClusterAdvertise != "" {
		return fmt.Errorf("--cluster-store and --cluster-advertise daemon configurations are incompatible with swarm mode")
	}
	if conf.LiveRestoreEnabled {
		return fmt.Errorf("--live-restore daemon configuration is incompatible with swarm mode")
	}
	return nil
}

func verifyDefaultIpcMode(mode string) error {
	const hint = "Use \"shareable\" or \"private\"."

	dm := containertypes.IpcMode(mode)
	if !dm.Valid() {
		return fmt.Errorf("Default IPC mode setting (%v) is invalid. "+hint, dm)
	}
	if dm != "" && !dm.IsPrivate() && !dm.IsShareable() {
		return fmt.Errorf("IPC mode \"%v\" is not supported as default value. "+hint, dm)
	}
	return nil
}

// ValidatePlatformConfig checks if any platform-specific configuration settings are invalid.
func (conf *Config) ValidatePlatformConfig() error {
	return verifyDefaultIpcMode(conf.IpcMode)
}
