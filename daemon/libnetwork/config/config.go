package config

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/cluster"
	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamutils"
	"github.com/moby/moby/v2/pkg/plugingetter"
)

const (
	warningThNetworkControlPlaneMTU = 1500
	minimumNetworkControlPlaneMTU   = 500
)

// Config encapsulates configurations of various Libnetwork components
type Config struct {
	PlatformConfig

	DataDir string
	// ExecRoot is the base-path for libnetwork external key listeners
	// (created in "<ExecRoot>/libnetwork/<Controller-Short-ID>.sock"),
	// and is passed as "-exec-root: argument for "libnetwork-setkey".
	//
	// It is only used on Linux, but referenced in some "unix" files
	// (linux and freebsd).
	//
	// FIXME(thaJeztah): ExecRoot is only used for Controller.startExternalKeyListener(), but "libnetwork-setkey" is only implemented on Linux.
	ExecRoot               string
	DefaultNetwork         string
	DefaultDriver          string
	Labels                 []string
	ClusterProvider        cluster.Provider
	NetworkControlPlaneMTU int
	DefaultAddressPool     []*ipamutils.NetworkToSplit
	// TODO(aker): make this a non-pointer once the feature flag 'global-default-subnet-size' is removed.
	DefaultSubnetSize   *int
	DatastoreBucket     string
	ActiveSandboxes     map[string]any
	PluginGetter        plugingetter.PluginGetter
	FirewallBackend     string
	Rootless            bool
	EnableUserlandProxy bool
	UserlandProxyPath   string
}

// New creates a new Config and initializes it with the given Options.
func New(opts ...Option) *Config {
	cfg := &Config{
		DatastoreBucket: datastore.DefaultBucket,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	return cfg
}

// Option is an option setter function type used to pass various configurations
// to the controller
type Option func(c *Config)

// OptionDefaultNetwork function returns an option setter for a default network
func OptionDefaultNetwork(dn string) Option {
	return func(c *Config) {
		log.G(context.TODO()).Debugf("Option DefaultNetwork: %s", dn)
		c.DefaultNetwork = strings.TrimSpace(dn)
	}
}

// OptionDefaultDriver function returns an option setter for default driver
func OptionDefaultDriver(dd string) Option {
	return func(c *Config) {
		log.G(context.TODO()).Debugf("Option DefaultDriver: %s", dd)
		c.DefaultDriver = strings.TrimSpace(dd)
	}
}

// OptionDefaultAddressPoolConfig function returns an option setter for default address pool
func OptionDefaultAddressPoolConfig(addressPool []*ipamutils.NetworkToSplit) Option {
	return func(c *Config) {
		c.DefaultAddressPool = addressPool
	}
}

// OptionDefaultSubnetSize defines a default subnet size that should be used for all dynamic subnet allocation. When
// this option is not set, each address pool has its own default subnet size — dynamic subnet allocation doesn't yield
// subnets of the same size across different address pools.
func OptionDefaultSubnetSize(size int) Option {
	return func(c *Config) {
		c.DefaultSubnetSize = &size
	}
}

// OptionDataDir function returns an option setter for data folder
func OptionDataDir(dataDir string) Option {
	return func(c *Config) {
		c.DataDir = dataDir
	}
}

// OptionExecRoot function returns an option setter for exec root folder.
//
// On Linux, it sets both the controller's ExecRoot and osl.basePath, whereas
// on FreeBSD, it only sets the controller's ExecRoot. It is a no-op on other
// platforms.
func OptionExecRoot(execRoot string) Option {
	return optionExecRoot(execRoot)
}

// OptionPluginGetter returns a plugingetter for remote drivers.
func OptionPluginGetter(pg plugingetter.PluginGetter) Option {
	return func(c *Config) {
		c.PluginGetter = pg
	}
}

// OptionNetworkControlPlaneMTU function returns an option setter for control plane MTU
func OptionNetworkControlPlaneMTU(exp int) Option {
	return func(c *Config) {
		log.G(context.TODO()).Debugf("Network Control Plane MTU: %d", exp)
		if exp < warningThNetworkControlPlaneMTU {
			log.G(context.TODO()).Warnf("Received a MTU of %d, this value is very low, the network control plane can misbehave,"+
				" defaulting to minimum value (%d)", exp, minimumNetworkControlPlaneMTU)
			if exp < minimumNetworkControlPlaneMTU {
				exp = minimumNetworkControlPlaneMTU
			}
		}
		c.NetworkControlPlaneMTU = exp
	}
}

// OptionActiveSandboxes function returns an option setter for passing the sandboxes
// which were active during previous daemon life
func OptionActiveSandboxes(sandboxes map[string]any) Option {
	return func(c *Config) {
		c.ActiveSandboxes = sandboxes
	}
}

// OptionFirewallBackend returns an option setter for selection of the firewall backend.
func OptionFirewallBackend(val string) Option {
	return func(c *Config) {
		c.FirewallBackend = val
	}
}

// OptionRootless returns an option setter that indicates whether the daemon is
// running in rootless mode.
func OptionRootless(rootless bool) Option {
	return func(c *Config) {
		c.Rootless = rootless
	}
}

// OptionUserlandProxy returns an option setter that indicates whether the
// userland proxy is enabled, and sets the path to the proxy binary.
func OptionUserlandProxy(enabled bool, proxyPath string) Option {
	return func(c *Config) {
		c.EnableUserlandProxy = enabled
		c.UserlandProxyPath = proxyPath
	}
}
