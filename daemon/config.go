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
	AutoRestart    bool
	Bridge         bridgeConfig // Bridge holds bridge network specific configuration.
	Context        map[string][]string
	CorsHeaders    string
	DisableBridge  bool
	Dns            []string
	DnsSearch      []string
	EnableCors     bool
	ExecDriver     string
	ExecOptions    []string
	ExecRoot       string
	GraphDriver    string
	GraphOptions   []string
	Labels         []string
	LogConfig      runconfig.LogConfig
	Mtu            int
	Pidfile        string
	Root           string
	TrustKeyPath   string
	DefaultNetwork string
	NetworkKVStore string
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
	cmd.StringVar(&config.ExecDriver, []string{"e", "-exec-driver"}, defaultExec, usageFn("Exec driver to use"))
	cmd.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, usageFn("Set the containers network MTU"))
	cmd.BoolVar(&config.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, usageFn("Enable CORS headers in the remote API, this is deprecated by --api-cors-header"))
	cmd.StringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", usageFn("Set CORS headers in the remote API"))
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	cmd.Var(opts.NewListOptsRef(&config.Dns, opts.ValidateIPAddress), []string{"#dns", "-dns"}, usageFn("DNS server to use"))
	cmd.Var(opts.NewListOptsRef(&config.DnsSearch, opts.ValidateDNSSearch), []string{"-dns-search"}, usageFn("DNS search domains to use"))
	cmd.Var(opts.NewListOptsRef(&config.Labels, opts.ValidateLabel), []string{"-label"}, usageFn("Set key=value labels to the daemon"))
	cmd.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", usageFn("Default driver for container logs"))
	cmd.Var(opts.NewMapOpts(config.LogConfig.Config, nil), []string{"-log-opt"}, usageFn("Set log driver options"))
}
