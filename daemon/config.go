package daemon

import (
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/daemon/networkdriver/bridge"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/runconfig"
	"net"
	"strings"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// Config define the configuration of a docker daemon
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
// FIXME: separate runtime configuration from http api configuration
type Config struct {
	Bridge bridge.Config

	Pidfile              string
	Root                 string
	AutoRestart          bool
	Dns                  []string
	DnsSearch            []string
	GraphDriver          string
	GraphOptions         []string
	ExecDriver           string
	Mtu                  int
	SocketGroup          string
	EnableCors           bool
	CorsHeaders          string
	DisableNetwork       bool
	EnableSelinuxSupport bool
	Context              map[string][]string
	TrustKeyPath         string
	Labels               []string
	Ulimits              map[string]*ulimit.Ulimit
	LogConfig            runconfig.LogConfig
}

func setDaemonMap(names []string) {
	for _, opt := range names {
		opt = strings.TrimPrefix(opt, "#")
		flag.MapDaemonFlags[opt] = true
	}
}

func addDaemonStringVar(p *string, names []string, value string, usage string) {
	flag.StringVar(p, names, value, usage)
	setDaemonMap(names)
}

func addDaemonIntVar(p *int, names []string, value int, usage string) {
	flag.IntVar(p, names, value, usage)
	setDaemonMap(names)
}

func addDaemonBoolVar(p *bool, names []string, value bool, usage string) {
	flag.BoolVar(p, names, value, usage)
	setDaemonMap(names)
}

func addDaemonIPVar(value *net.IP, names []string, defaultValue, usage string) {
	opts.IPVar(value, names, defaultValue, usage)
	setDaemonMap(names)
}

func addDaemonListVar(values *[]string, names []string, usage string) {
	opts.ListVar(values, names, usage)
	setDaemonMap(names)
}

func addDaemonIPListVar(values *[]string, names []string, usage string) {
	opts.IPListVar(values, names, usage)
	setDaemonMap(names)
}

func addDaemonDnsSearchListVar(values *[]string, names []string, usage string) {
	opts.DnsSearchListVar(values, names, usage)
	setDaemonMap(names)
}

func addDaemonLabelListVar(values *[]string, names []string, usage string) {
	opts.LabelListVar(values, names, usage)
	setDaemonMap(names)
}

func (config *Config) addDaemonUlimitMapVar(names []string, usage string) {
	config.Ulimits = make(map[string]*ulimit.Ulimit)
	opts.UlimitMapVar(config.Ulimits, names, usage)
	setDaemonMap(names)
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallFlags() {
	addDaemonStringVar(&config.Pidfile, []string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	addDaemonStringVar(&config.Root, []string{"g", "-graph"}, "/var/lib/docker", "Root of the Docker runtime")
	addDaemonBoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	addDaemonBoolVar(&config.Bridge.EnableIptables, []string{"#iptables", "-iptables"}, true, "Enable addition of iptables rules")
	addDaemonBoolVar(&config.Bridge.EnableIpForward, []string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	addDaemonBoolVar(&config.Bridge.EnableIpMasq, []string{"-ip-masq"}, true, "Enable IP masquerading")
	addDaemonBoolVar(&config.Bridge.EnableIPv6, []string{"-ipv6"}, false, "Enable IPv6 networking")
	addDaemonStringVar(&config.Bridge.IP, []string{"#bip", "-bip"}, "", "Specify network bridge IP")
	addDaemonStringVar(&config.Bridge.Iface, []string{"b", "-bridge"}, "", "Attach containers to a network bridge")
	addDaemonStringVar(&config.Bridge.FixedCIDR, []string{"-fixed-cidr"}, "", "IPv4 subnet for fixed IPs")
	addDaemonStringVar(&config.Bridge.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", "IPv6 subnet for fixed IPs")
	addDaemonStringVar(&config.Bridge.DefaultGatewayIPv4, []string{"-default-gateway"}, "", "Container default gateway IPv4 address")
	addDaemonStringVar(&config.Bridge.DefaultGatewayIPv6, []string{"-default-gateway-v6"}, "", "Container default gateway IPv6 address")
	addDaemonBoolVar(&config.Bridge.InterContainerCommunication, []string{"#icc", "-icc"}, true, "Enable inter-container communication")
	addDaemonStringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", "Storage driver to use")
	addDaemonStringVar(&config.ExecDriver, []string{"e", "-exec-driver"}, "native", "Exec driver to use")
	addDaemonBoolVar(&config.EnableSelinuxSupport, []string{"-selinux-enabled"}, false, "Enable selinux support")
	addDaemonIntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, "Set the containers network MTU")
	addDaemonStringVar(&config.SocketGroup, []string{"G", "-group"}, "docker", "Group for the unix socket")
	addDaemonBoolVar(&config.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	addDaemonStringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", "Set CORS headers in the remote API")

	addDaemonIPVar(&config.Bridge.DefaultIp, []string{"#ip", "-ip"}, "0.0.0.0", "Default IP when binding container ports")
	addDaemonListVar(&config.GraphOptions, []string{"-storage-opt"}, "Set storage driver options")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	addDaemonIPListVar(&config.Dns, []string{"#dns", "-dns"}, "DNS server to use")
	addDaemonDnsSearchListVar(&config.DnsSearch, []string{"-dns-search"}, "DNS search domains to use")
	addDaemonLabelListVar(&config.Labels, []string{"-label"}, "Set key=value labels to the daemon")
	config.addDaemonUlimitMapVar([]string{"-default-ulimit"}, "Set default ulimits for containers")
	addDaemonStringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", "Containers logging driver")
}

func getDefaultNetworkMtu() int {
	if iface, err := networkdriver.GetDefaultRouteIface(); err == nil {
		return iface.MTU
	}
	return defaultNetworkMtu
}
