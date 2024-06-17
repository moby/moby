// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.21

package config

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/pkg/plugingetter"
)

const (
	warningThNetworkControlPlaneMTU = 1500
	minimumNetworkControlPlaneMTU   = 500
)

// Config encapsulates configurations of various Libnetwork components
type Config struct {
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
	driverCfg              map[string]map[string]any
	ClusterProvider        cluster.Provider
	NetworkControlPlaneMTU int
	DefaultAddressPool     []*ipamutils.NetworkToSplit
	Scope                  datastore.ScopeCfg
	ActiveSandboxes        map[string]interface{}
	PluginGetter           plugingetter.PluginGetter
}

// New creates a new Config and initializes it with the given Options.
func New(opts ...Option) *Config {
	cfg := &Config{
		driverCfg: make(map[string]map[string]any),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	// load default scope configs which don't have explicit user specified configs.
	if cfg.Scope == (datastore.ScopeCfg{}) {
		cfg.Scope = datastore.DefaultScope(cfg.DataDir)
	}
	return cfg
}

func (c *Config) DriverConfig(name string) map[string]any {
	return c.driverCfg[name]
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

// OptionDriverConfig returns an option setter for driver configuration.
func OptionDriverConfig(networkType string, config map[string]any) Option {
	return func(c *Config) {
		c.driverCfg[networkType] = config
	}
}

// OptionLabels function returns an option setter for labels
func OptionLabels(labels []string) Option {
	return func(c *Config) {
		for _, label := range labels {
			if strings.HasPrefix(label, netlabel.Prefix) {
				c.Labels = append(c.Labels, label)
			}
		}
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
func OptionActiveSandboxes(sandboxes map[string]interface{}) Option {
	return func(c *Config) {
		c.ActiveSandboxes = sandboxes
	}
}
