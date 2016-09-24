// +build linux freebsd

package daemon

import (
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"
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

	// Fields below here are platform specific.
	CgroupParent         string                   `json:"cgroup-parent,omitempty"`
	ContainerdAddr       string                   `json:"containerd,omitempty"`
	EnableSelinuxSupport bool                     `json:"selinux-enabled,omitempty"`
	ExecRoot             string                   `json:"exec-root,omitempty"`
	RemappedRoot         string                   `json:"userns-remap,omitempty"`
	Ulimits              map[string]*units.Ulimit `json:"default-ulimits,omitempty"`
	Runtimes             map[string]types.Runtime `json:"runtimes,omitempty"`
	DefaultRuntime       string                   `json:"default-runtime,omitempty"`
	OOMScoreAdjust       int                      `json:"oom-score-adjust,omitempty"`
	Init                 bool                     `json:"init,omitempty"`
	InitPath             string                   `json:"init-path,omitempty"`
}

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig struct {
	commonBridgeConfig

	// Fields below here are platform specific.
	EnableIPv6                  bool   `json:"ipv6,omitempty"`
	EnableIPTables              bool   `json:"iptables,omitempty"`
	EnableIPForward             bool   `json:"ip-forward,omitempty"`
	EnableIPMasq                bool   `json:"ip-masq,omitempty"`
	EnableUserlandProxy         bool   `json:"userland-proxy,omitempty"`
	UserlandProxyPath           string `json:"userland-proxy-path,omitempty"`
	DefaultIP                   net.IP `json:"ip,omitempty"`
	IP                          string `json:"bip,omitempty"`
	FixedCIDRv6                 string `json:"fixed-cidr-v6,omitempty"`
	DefaultGatewayIPv4          net.IP `json:"default-gateway,omitempty"`
	DefaultGatewayIPv6          net.IP `json:"default-gateway-v6,omitempty"`
	InterContainerCommunication bool   `json:"icc,omitempty"`
}

// InstallFlags adds flags to the pflag.FlagSet to configure the daemon
func (config *Config) InstallFlags(flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags(flags)

	config.Ulimits = make(map[string]*units.Ulimit)
	config.Runtimes = make(map[string]types.Runtime)

	// Then platform-specific install flags
	flags.BoolVar(&config.EnableSelinuxSupport, "selinux-enabled", false, "Enable selinux support")
	flags.StringVarP(&config.SocketGroup, "group", "G", "docker", "Group for the unix socket")
	flags.Var(runconfigopts.NewUlimitOpt(&config.Ulimits), "default-ulimit", "Default ulimits for containers")
	flags.BoolVar(&config.bridgeConfig.EnableIPTables, "iptables", true, "Enable addition of iptables rules")
	flags.BoolVar(&config.bridgeConfig.EnableIPForward, "ip-forward", true, "Enable net.ipv4.ip_forward")
	flags.BoolVar(&config.bridgeConfig.EnableIPMasq, "ip-masq", true, "Enable IP masquerading")
	flags.BoolVar(&config.bridgeConfig.EnableIPv6, "ipv6", false, "Enable IPv6 networking")
	flags.StringVar(&config.ExecRoot, "exec-root", defaultExecRoot, "Root directory for execution state files")
	flags.StringVar(&config.bridgeConfig.IP, "bip", "", "Specify network bridge IP")
	flags.StringVarP(&config.bridgeConfig.Iface, "bridge", "b", "", "Attach containers to a network bridge")
	flags.StringVar(&config.bridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.StringVar(&config.bridgeConfig.FixedCIDRv6, "fixed-cidr-v6", "", "IPv6 subnet for fixed IPs")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultGatewayIPv4, ""), "default-gateway", "Container default gateway IPv4 address")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultGatewayIPv6, ""), "default-gateway-v6", "Container default gateway IPv6 address")
	flags.BoolVar(&config.bridgeConfig.InterContainerCommunication, "icc", true, "Enable inter-container communication")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultIP, "0.0.0.0"), "ip", "Default IP when binding container ports")
	flags.BoolVar(&config.bridgeConfig.EnableUserlandProxy, "userland-proxy", true, "Use userland proxy for loopback traffic")
	flags.StringVar(&config.bridgeConfig.UserlandProxyPath, "userland-proxy-path", "", "Path to the userland proxy binary")
	flags.BoolVar(&config.EnableCors, "api-enable-cors", false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	flags.MarkDeprecated("api-enable-cors", "Please use --api-cors-header")
	flags.StringVar(&config.CgroupParent, "cgroup-parent", "", "Set parent cgroup for all containers")
	flags.StringVar(&config.RemappedRoot, "userns-remap", "", "User/Group setting for user namespaces")
	flags.StringVar(&config.ContainerdAddr, "containerd", "", "Path to containerd socket")
	flags.BoolVar(&config.LiveRestoreEnabled, "live-restore", false, "Enable live restore of docker when containers are still running")
	flags.Var(runconfigopts.NewNamedRuntimeOpt("runtimes", &config.Runtimes, stockRuntimeName), "add-runtime", "Register an additional OCI compatible runtime")
	flags.StringVar(&config.DefaultRuntime, "default-runtime", stockRuntimeName, "Default OCI runtime for containers")
	flags.IntVar(&config.OOMScoreAdjust, "oom-score-adjust", -500, "Set the oom_score_adj for the daemon")
	flags.BoolVar(&config.Init, "init", false, "Run an init in the container to forward signals and reap processes")
	flags.StringVar(&config.InitPath, "init-path", "", "Path to the docker-init binary")

	config.attachExperimentalFlags(flags)
}

// GetRuntime returns the runtime path and arguments for a given
// runtime name
func (config *Config) GetRuntime(name string) *types.Runtime {
	config.reloadLock.Lock()
	defer config.reloadLock.Unlock()
	if rt, ok := config.Runtimes[name]; ok {
		return &rt
	}
	return nil
}

// GetDefaultRuntimeName returns the current default runtime
func (config *Config) GetDefaultRuntimeName() string {
	config.reloadLock.Lock()
	rt := config.DefaultRuntime
	config.reloadLock.Unlock()

	return rt
}

// GetAllRuntimes returns a copy of the runtimes map
func (config *Config) GetAllRuntimes() map[string]types.Runtime {
	config.reloadLock.Lock()
	rts := config.Runtimes
	config.reloadLock.Unlock()
	return rts
}

// GetExecRoot returns the user configured Exec-root
func (config *Config) GetExecRoot() string {
	return config.ExecRoot
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
