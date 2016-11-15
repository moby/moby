// +build linux freebsd

package daemon

import (
	"fmt"

	runconfigopts "github.com/docker/docker/runconfig/opts"
	units "github.com/docker/go-units"
	"github.com/spf13/pflag"
)

var (
	defaultPidFile  = "/var/run/docker.pid"
	defaultGraph    = "/var/lib/docker"
	defaultExecRoot = "/var/run/docker"
)

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// These fields are common to all unix platforms.
	CommonUnixConfig

	// Fields below here are platform specific.
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
}

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig struct {
	commonBridgeConfig

	// These fields are common to all unix platforms.
	commonUnixBridgeConfig

	// Fields below here are platform specific.
	EnableIPv6          bool   `json:"ipv6,omitempty"`
	EnableIPTables      bool   `json:"iptables,omitempty"`
	EnableIPForward     bool   `json:"ip-forward,omitempty"`
	EnableIPMasq        bool   `json:"ip-masq,omitempty"`
	EnableUserlandProxy bool   `json:"userland-proxy,omitempty"`
	UserlandProxyPath   string `json:"userland-proxy-path,omitempty"`
	FixedCIDRv6         string `json:"fixed-cidr-v6,omitempty"`
}

// InstallFlags adds flags to the pflag.FlagSet to configure the daemon
func (config *Config) InstallFlags(flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags(flags)

	// Then install flags common to unix platforms
	config.InstallCommonUnixFlags(flags)

	config.Ulimits = make(map[string]*units.Ulimit)

	// Then platform-specific install flags
	flags.BoolVar(&config.EnableSelinuxSupport, "selinux-enabled", false, "Enable selinux support")
	flags.Var(runconfigopts.NewUlimitOpt(&config.Ulimits), "default-ulimit", "Default ulimits for containers")
	flags.BoolVar(&config.bridgeConfig.EnableIPTables, "iptables", true, "Enable addition of iptables rules")
	flags.BoolVar(&config.bridgeConfig.EnableIPForward, "ip-forward", true, "Enable net.ipv4.ip_forward")
	flags.BoolVar(&config.bridgeConfig.EnableIPMasq, "ip-masq", true, "Enable IP masquerading")
	flags.BoolVar(&config.bridgeConfig.EnableIPv6, "ipv6", false, "Enable IPv6 networking")
	flags.StringVar(&config.ExecRoot, "exec-root", defaultExecRoot, "Root directory for execution state files")
	flags.StringVar(&config.bridgeConfig.FixedCIDRv6, "fixed-cidr-v6", "", "IPv6 subnet for fixed IPs")
	flags.BoolVar(&config.bridgeConfig.EnableUserlandProxy, "userland-proxy", true, "Use userland proxy for loopback traffic")
	flags.StringVar(&config.bridgeConfig.UserlandProxyPath, "userland-proxy-path", "", "Path to the userland proxy binary")
	flags.BoolVar(&config.EnableCors, "api-enable-cors", false, "Enable CORS headers in the Engine API, this is deprecated by --api-cors-header")
	flags.MarkDeprecated("api-enable-cors", "Please use --api-cors-header")
	flags.StringVar(&config.CgroupParent, "cgroup-parent", "", "Set parent cgroup for all containers")
	flags.StringVar(&config.RemappedRoot, "userns-remap", "", "User/Group setting for user namespaces")
	flags.StringVar(&config.ContainerdAddr, "containerd", "", "Path to containerd socket")
	flags.BoolVar(&config.LiveRestoreEnabled, "live-restore", false, "Enable live restore of docker when containers are still running")
	flags.IntVar(&config.OOMScoreAdjust, "oom-score-adjust", -500, "Set the oom_score_adj for the daemon")
	flags.BoolVar(&config.Init, "init", false, "Run an init in the container to forward signals and reap processes")
	flags.StringVar(&config.InitPath, "init-path", "", "Path to the docker-init binary")
	flags.Int64Var(&config.CPURealtimePeriod, "cpu-rt-period", 0, "Limit the CPU real-time period in microseconds")
	flags.Int64Var(&config.CPURealtimeRuntime, "cpu-rt-runtime", 0, "Limit the CPU real-time runtime in microseconds")
	flags.StringVar(&config.SeccompProfile, "seccomp-profile", "", "Path to seccomp profile")

	config.attachExperimentalFlags(flags)
}

func (config *Config) isSwarmCompatible() error {
	if config.ClusterStore != "" || config.ClusterAdvertise != "" {
		return fmt.Errorf("--cluster-store and --cluster-advertise daemon configurations are incompatible with swarm mode")
	}
	if config.LiveRestoreEnabled {
		return fmt.Errorf("--live-restore daemon configuration is incompatible with swarm mode")
	}
	return nil
}
