package daemon

import (
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
	AutoRestart   bool
	Bridge        bridgeConfig // Bridge holds bridge network specific configuration.
	Context       map[string][]string
	DisableBridge bool
	DNS           []string
	DNSOptions    []string
	DNSSearch     []string
	ExecOptions   []string
	ExecRoot      string
	GraphDriver   string
	GraphOptions  []string
	Labels        []string
	LogConfig     runconfig.LogConfig
	Mtu           int
	Pidfile       string
	RemappedRoot  string
	Root          string
	TrustKeyPath  string

	// ClusterStore is the storage backend used for the cluster information. It is used by both
	// multihost networking (to store networks and endpoints information) and by the node discovery
	// mechanism.
	ClusterStore string

	// ClusterOpts is used to pass options to the discovery package for tuning libkv settings, such
	// as TLS configuration settings.
	ClusterOpts map[string]string

	// ClusterAdvertise is the network endpoint that the Engine advertises for the purpose of node
	// discovery. This should be a 'host:port' combination on which that daemon instance is
	// reachable by other hosts.
	ClusterAdvertise string
}

// InstallCommonFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallCommonFlags(cmd *flag.FlagSet, usageFn func(string) string) {
	cmd.Var(opts.NewListOptsRef(&config.GraphOptions, nil), []string{"-storage-opt"}, usageFn("Set storage driver options"))
	cmd.Var(opts.NewListOptsRef(&config.ExecOptions, nil), []string{"-exec-opt"}, usageFn("Set exec driver options"))
	cmd.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, defaultPidFile, usageFn("Path to use for daemon PID file"))
	cmd.StringVar(&config.Root, []string{"g", "-graph"}, defaultGraph, usageFn("Root of the Docker runtime"))
	cmd.StringVar(&config.ExecRoot, []string{"-exec-root"}, "/var/run/docker", usageFn("Root of the Docker execdriver"))
	cmd.BoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, usageFn("--restart on the daemon has been deprecated in favor of --restart policies on docker run"))
	cmd.StringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", usageFn("Storage driver to use"))
	cmd.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, usageFn("Set the containers network MTU"))
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	cmd.Var(opts.NewListOptsRef(&config.DNS, opts.ValidateIPAddress), []string{"#dns", "-dns"}, usageFn("DNS server to use"))
	cmd.Var(opts.NewListOptsRef(&config.DNSOptions, nil), []string{"-dns-opt"}, usageFn("DNS options to use"))
	cmd.Var(opts.NewListOptsRef(&config.DNSSearch, opts.ValidateDNSSearch), []string{"-dns-search"}, usageFn("DNS search domains to use"))
	cmd.Var(opts.NewListOptsRef(&config.Labels, opts.ValidateLabel), []string{"-label"}, usageFn("Set key=value labels to the daemon"))
	cmd.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", usageFn("Default driver for container logs"))
	cmd.Var(opts.NewMapOpts(config.LogConfig.Config, nil), []string{"-log-opt"}, usageFn("Set log driver options"))
	cmd.StringVar(&config.ClusterAdvertise, []string{"-cluster-advertise"}, "", usageFn("Address or interface name to advertise"))
	cmd.StringVar(&config.ClusterStore, []string{"-cluster-store"}, "", usageFn("Set the cluster store"))
	cmd.Var(opts.NewMapOpts(config.ClusterOpts, nil), []string{"-cluster-store-opt"}, usageFn("Set cluster store options"))
}
