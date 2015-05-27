package daemon

import (
	"net"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/runconfig"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// CommonConfig defines the configuration of a docker daemon which are
// common across platforms.
type CommonConfig struct {
	AutoRestart bool
	// Bridge holds bridge network specific configuration.
	Bridge         bridgeConfig
	Context        map[string][]string
	CorsHeaders    string
	DisableNetwork bool
	Dns            []string
	DnsSearch      []string
	EnableCors     bool
	ExecDriver     string
	ExecRoot       string
	GraphDriver    string
	Labels         []string
	LogConfig      runconfig.LogConfig
	Mtu            int
	Pidfile        string
	Root           string
	TrustKeyPath   string
}

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig struct {
	EnableIPv6                  bool
	EnableIPTables              bool
	EnableIPForward             bool
	EnableIPMasq                bool
	EnableUserlandProxy         bool
	DefaultIP                   net.IP
	Iface                       string
	IP                          string
	FixedCIDR                   string
	FixedCIDRv6                 string
	DefaultGatewayIPv4          string
	DefaultGatewayIPv6          string
	InterContainerCommunication bool
}

// InstallCommonFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.

func (config *Config) InstallCommonFlags() {
	flag.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, defaultPidFile, "Path to use for daemon PID file")
	flag.StringVar(&config.Root, []string{"g", "-graph"}, defaultGraph, "Root of the Docker runtime")
	flag.StringVar(&config.ExecRoot, []string{"-exec-root"}, "/var/run/docker", "Root of the Docker execdriver")
	flag.BoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	flag.BoolVar(&config.Bridge.EnableIPTables, []string{"#iptables", "-iptables"}, true, "Enable addition of iptables rules")
	flag.BoolVar(&config.Bridge.EnableIPForward, []string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	flag.BoolVar(&config.Bridge.EnableIPMasq, []string{"-ip-masq"}, true, "Enable IP masquerading")
	flag.BoolVar(&config.Bridge.EnableIPv6, []string{"-ipv6"}, false, "Enable IPv6 networking")
	flag.StringVar(&config.Bridge.IP, []string{"#bip", "-bip"}, "", "Specify network bridge IP")
	flag.StringVar(&config.Bridge.Iface, []string{"b", "-bridge"}, "", "Attach containers to a network bridge")
	flag.StringVar(&config.Bridge.FixedCIDR, []string{"-fixed-cidr"}, "", "IPv4 subnet for fixed IPs")
	flag.StringVar(&config.Bridge.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", "IPv6 subnet for fixed IPs")
	flag.StringVar(&config.Bridge.DefaultGatewayIPv4, []string{"-default-gateway"}, "", "Container default gateway IPv4 address")
	flag.StringVar(&config.Bridge.DefaultGatewayIPv6, []string{"-default-gateway-v6"}, "", "Container default gateway IPv6 address")
	flag.BoolVar(&config.Bridge.InterContainerCommunication, []string{"#icc", "-icc"}, true, "Enable inter-container communication")
	flag.StringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", "Storage driver to use")
	flag.StringVar(&config.ExecDriver, []string{"e", "-exec-driver"}, defaultExec, "Exec driver to use")
	flag.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, "Set the containers network MTU")
	flag.BoolVar(&config.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	flag.StringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", "Set CORS headers in the remote API")
	opts.IPVar(&config.Bridge.DefaultIP, []string{"#ip", "-ip"}, "0.0.0.0", "Default IP when binding container ports")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	opts.IPListVar(&config.Dns, []string{"#dns", "-dns"}, "DNS server to use")
	opts.DnsSearchListVar(&config.DnsSearch, []string{"-dns-search"}, "DNS search domains to use")
	opts.LabelListVar(&config.Labels, []string{"-label"}, "Set key=value labels to the daemon")
	flag.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", "Default driver for container logs")
	opts.LogOptsVar(config.LogConfig.Config, []string{"-log-opt"}, "Set log driver options")
	flag.BoolVar(&config.Bridge.EnableUserlandProxy, []string{"-userland-proxy"}, true, "Use userland proxy for loopback traffic")

}
