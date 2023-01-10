//go:build linux || freebsd
// +build linux freebsd

package main

import (
	"net"
	"path/filepath"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/pkg/rootless"
	"github.com/docker/docker/registry"
	"github.com/spf13/pflag"
)

// installConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) error {
	// First handle install flags which are consistent cross-platform
	if err := installCommonConfigFlags(conf, flags); err != nil {
		return err
	}

	// Then platform-specific install flags
	flags.Var(opts.NewNamedRuntimeOpt("runtimes", &conf.Runtimes, config.StockRuntimeName), "add-runtime", "Register an additional OCI compatible runtime")
	flags.StringVarP(&conf.SocketGroup, "group", "G", "docker", "Group for the unix socket")
	flags.StringVarP(&conf.GraphDriver, "storage-driver", "s", "", "Storage driver to use")
	flags.BoolVar(&conf.EnableSelinuxSupport, "selinux-enabled", false, "Enable selinux support")
	flags.Var(opts.NewNamedUlimitOpt("default-ulimits", &conf.Ulimits), "default-ulimit", "Default ulimits for containers")
	flags.BoolVar(&conf.BridgeConfig.EnableIPTables, "iptables", true, "Enable addition of iptables rules")
	flags.BoolVar(&conf.BridgeConfig.EnableIP6Tables, "ip6tables", false, "Enable addition of ip6tables rules (experimental)")
	flags.BoolVar(&conf.BridgeConfig.EnableIPForward, "ip-forward", true, "Enable net.ipv4.ip_forward")
	flags.BoolVar(&conf.BridgeConfig.EnableIPMasq, "ip-masq", true, "Enable IP masquerading")
	flags.BoolVar(&conf.BridgeConfig.EnableIPv6, "ipv6", false, "Enable IPv6 networking")
	flags.StringVar(&conf.BridgeConfig.IP, "bip", "", "Specify network bridge IP")
	flags.StringVarP(&conf.BridgeConfig.Iface, "bridge", "b", "", "Attach containers to a network bridge")
	flags.StringVar(&conf.BridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.StringVar(&conf.BridgeConfig.FixedCIDRv6, "fixed-cidr-v6", "", "IPv6 subnet for fixed IPs")
	flags.IPVar(&conf.BridgeConfig.DefaultGatewayIPv4, "default-gateway", nil, "Container default gateway IPv4 address")
	flags.IPVar(&conf.BridgeConfig.DefaultGatewayIPv6, "default-gateway-v6", nil, "Container default gateway IPv6 address")
	flags.BoolVar(&conf.BridgeConfig.InterContainerCommunication, "icc", true, "Enable inter-container communication")
	flags.IPVar(&conf.BridgeConfig.DefaultIP, "ip", net.IPv4zero, "Default IP when binding container ports")
	flags.BoolVar(&conf.BridgeConfig.EnableUserlandProxy, "userland-proxy", true, "Use userland proxy for loopback traffic")
	flags.StringVar(&conf.BridgeConfig.UserlandProxyPath, "userland-proxy-path", conf.BridgeConfig.UserlandProxyPath, "Path to the userland proxy binary")
	flags.StringVar(&conf.CgroupParent, "cgroup-parent", "", "Set parent cgroup for all containers")
	flags.StringVar(&conf.RemappedRoot, "userns-remap", "", "User/Group setting for user namespaces")
	flags.BoolVar(&conf.LiveRestoreEnabled, "live-restore", false, "Enable live restore of docker when containers are still running")
	flags.IntVar(&conf.OOMScoreAdjust, "oom-score-adjust", 0, "Set the oom_score_adj for the daemon")
	flags.BoolVar(&conf.Init, "init", false, "Run an init in the container to forward signals and reap processes")
	flags.StringVar(&conf.InitPath, "init-path", "", "Path to the docker-init binary")
	flags.Int64Var(&conf.CPURealtimePeriod, "cpu-rt-period", 0, "Limit the CPU real-time period in microseconds for the parent cgroup for all containers (not supported with cgroups v2)")
	flags.Int64Var(&conf.CPURealtimeRuntime, "cpu-rt-runtime", 0, "Limit the CPU real-time runtime in microseconds for the parent cgroup for all containers (not supported with cgroups v2)")
	flags.StringVar(&conf.SeccompProfile, "seccomp-profile", conf.SeccompProfile, `Path to seccomp profile. Use "unconfined" to disable the default seccomp profile`)
	flags.Var(&conf.ShmSize, "default-shm-size", "Default shm size for containers")
	flags.BoolVar(&conf.NoNewPrivileges, "no-new-privileges", false, "Set no-new-privileges by default for new containers")
	flags.StringVar(&conf.IpcMode, "default-ipc-mode", conf.IpcMode, `Default mode for containers ipc ("shareable" | "private")`)
	flags.Var(&conf.NetworkConfig.DefaultAddressPools, "default-address-pool", "Default address pools for node specific local networks")
	// rootless needs to be explicitly specified for running "rootful" dockerd in rootless dockerd (#38702)
	// Note that conf.BridgeConfig.UserlandProxyPath and honorXDG are configured according to the value of rootless.RunningWithRootlessKit, not the value of --rootless.
	flags.BoolVar(&conf.Rootless, "rootless", conf.Rootless, "Enable rootless mode; typically used with RootlessKit")
	flags.StringVar(&conf.CgroupNamespaceMode, "default-cgroupns-mode", conf.CgroupNamespaceMode, `Default mode for containers cgroup namespace ("host" | "private")`)
	return nil
}

// configureCertsDir configures registry.CertsDir() depending on if the daemon
// is running in rootless mode or not.
func configureCertsDir() {
	if rootless.RunningWithRootlessKit() {
		configHome, err := homedir.GetConfigHome()
		if err == nil {
			registry.SetCertsDir(filepath.Join(configHome, "docker/certs.d"))
		}
	}
}
