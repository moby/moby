package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/portallocator"
	"github.com/docker/docker/pkg/discovery"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/libkv/store"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
)

const (
	warningThNetworkControlPlaneMTU = 1500
	minimumNetworkControlPlaneMTU   = 500
)

// Config encapsulates configurations of various Libnetwork components
type Config struct {
	Daemon          DaemonCfg
	Cluster         ClusterCfg
	Scopes          map[string]*datastore.ScopeCfg
	ActiveSandboxes map[string]interface{}
	PluginGetter    plugingetter.PluginGetter
}

// DaemonCfg represents libnetwork core configuration
type DaemonCfg struct {
	Debug                  bool
	Experimental           bool
	DataDir                string
	ExecRoot               string
	DefaultNetwork         string
	DefaultDriver          string
	Labels                 []string
	DriverCfg              map[string]interface{}
	ClusterProvider        cluster.Provider
	NetworkControlPlaneMTU int
	DefaultAddressPool     []*ipamutils.NetworkToSplit
}

// ClusterCfg represents cluster configuration
type ClusterCfg struct {
	Watcher   discovery.Watcher
	Address   string
	Discovery string
	Heartbeat uint64
}

// LoadDefaultScopes loads default scope configs for scopes which
// doesn't have explicit user specified configs.
func (c *Config) LoadDefaultScopes(dataDir string) {
	for k, v := range datastore.DefaultScopes(dataDir) {
		if _, ok := c.Scopes[k]; !ok {
			c.Scopes[k] = v
		}
	}
}

// ParseConfig parses the libnetwork configuration file
func ParseConfig(tomlCfgFile string) (*Config, error) {
	cfg := &Config{
		Scopes: map[string]*datastore.ScopeCfg{},
	}
	data, err := os.ReadFile(tomlCfgFile)
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.LoadDefaultScopes(cfg.Daemon.DataDir)
	return cfg, nil
}

// ParseConfigOptions parses the configuration options and returns
// a reference to the corresponding Config structure
func ParseConfigOptions(cfgOptions ...Option) *Config {
	cfg := &Config{
		Daemon: DaemonCfg{
			DriverCfg: make(map[string]interface{}),
		},
		Scopes: make(map[string]*datastore.ScopeCfg),
	}

	cfg.ProcessOptions(cfgOptions...)
	cfg.LoadDefaultScopes(cfg.Daemon.DataDir)

	return cfg
}

// Option is an option setter function type used to pass various configurations
// to the controller
type Option func(c *Config)

// OptionDefaultNetwork function returns an option setter for a default network
func OptionDefaultNetwork(dn string) Option {
	return func(c *Config) {
		logrus.Debugf("Option DefaultNetwork: %s", dn)
		c.Daemon.DefaultNetwork = strings.TrimSpace(dn)
	}
}

// OptionDefaultDriver function returns an option setter for default driver
func OptionDefaultDriver(dd string) Option {
	return func(c *Config) {
		logrus.Debugf("Option DefaultDriver: %s", dd)
		c.Daemon.DefaultDriver = strings.TrimSpace(dd)
	}
}

// OptionDefaultAddressPoolConfig function returns an option setter for default address pool
func OptionDefaultAddressPoolConfig(addressPool []*ipamutils.NetworkToSplit) Option {
	return func(c *Config) {
		c.Daemon.DefaultAddressPool = addressPool
	}
}

// OptionDriverConfig returns an option setter for driver configuration.
func OptionDriverConfig(networkType string, config map[string]interface{}) Option {
	return func(c *Config) {
		c.Daemon.DriverCfg[networkType] = config
	}
}

// OptionLabels function returns an option setter for labels
func OptionLabels(labels []string) Option {
	return func(c *Config) {
		for _, label := range labels {
			if strings.HasPrefix(label, netlabel.Prefix) {
				c.Daemon.Labels = append(c.Daemon.Labels, label)
			}
		}
	}
}

// OptionKVProvider function returns an option setter for kvstore provider
func OptionKVProvider(provider string) Option {
	return func(c *Config) {
		logrus.Debugf("Option OptionKVProvider: %s", provider)
		if _, ok := c.Scopes[datastore.GlobalScope]; !ok {
			c.Scopes[datastore.GlobalScope] = &datastore.ScopeCfg{}
		}
		c.Scopes[datastore.GlobalScope].Client.Provider = strings.TrimSpace(provider)
	}
}

// OptionKVProviderURL function returns an option setter for kvstore url
func OptionKVProviderURL(url string) Option {
	return func(c *Config) {
		logrus.Debugf("Option OptionKVProviderURL: %s", url)
		if _, ok := c.Scopes[datastore.GlobalScope]; !ok {
			c.Scopes[datastore.GlobalScope] = &datastore.ScopeCfg{}
		}
		c.Scopes[datastore.GlobalScope].Client.Address = strings.TrimSpace(url)
	}
}

// OptionKVOpts function returns an option setter for kvstore options
func OptionKVOpts(opts map[string]string) Option {
	return func(c *Config) {
		if opts["kv.cacertfile"] != "" && opts["kv.certfile"] != "" && opts["kv.keyfile"] != "" {
			logrus.Info("Option Initializing KV with TLS")
			tlsConfig, err := tlsconfig.Client(tlsconfig.Options{
				CAFile:   opts["kv.cacertfile"],
				CertFile: opts["kv.certfile"],
				KeyFile:  opts["kv.keyfile"],
			})
			if err != nil {
				logrus.Errorf("Unable to set up TLS: %s", err)
				return
			}
			if _, ok := c.Scopes[datastore.GlobalScope]; !ok {
				c.Scopes[datastore.GlobalScope] = &datastore.ScopeCfg{}
			}
			if c.Scopes[datastore.GlobalScope].Client.Config == nil {
				c.Scopes[datastore.GlobalScope].Client.Config = &store.Config{TLS: tlsConfig}
			} else {
				c.Scopes[datastore.GlobalScope].Client.Config.TLS = tlsConfig
			}
			// Workaround libkv/etcd bug for https
			c.Scopes[datastore.GlobalScope].Client.Config.ClientTLS = &store.ClientTLSConfig{
				CACertFile: opts["kv.cacertfile"],
				CertFile:   opts["kv.certfile"],
				KeyFile:    opts["kv.keyfile"],
			}
		} else {
			logrus.Info("Option Initializing KV without TLS")
		}
	}
}

// OptionDiscoveryWatcher function returns an option setter for discovery watcher
func OptionDiscoveryWatcher(watcher discovery.Watcher) Option {
	return func(c *Config) {
		c.Cluster.Watcher = watcher
	}
}

// OptionDiscoveryAddress function returns an option setter for self discovery address
func OptionDiscoveryAddress(address string) Option {
	return func(c *Config) {
		c.Cluster.Address = address
	}
}

// OptionDataDir function returns an option setter for data folder
func OptionDataDir(dataDir string) Option {
	return func(c *Config) {
		c.Daemon.DataDir = dataDir
	}
}

// OptionExecRoot function returns an option setter for exec root folder
func OptionExecRoot(execRoot string) Option {
	return func(c *Config) {
		c.Daemon.ExecRoot = execRoot
		osl.SetBasePath(execRoot)
	}
}

// OptionPluginGetter returns a plugingetter for remote drivers.
func OptionPluginGetter(pg plugingetter.PluginGetter) Option {
	return func(c *Config) {
		c.PluginGetter = pg
	}
}

// OptionExperimental function returns an option setter for experimental daemon
func OptionExperimental(exp bool) Option {
	return func(c *Config) {
		logrus.Debugf("Option Experimental: %v", exp)
		c.Daemon.Experimental = exp
	}
}

// OptionDynamicPortRange function returns an option setter for service port allocation range
func OptionDynamicPortRange(in string) Option {
	return func(c *Config) {
		start, end := 0, 0
		if len(in) > 0 {
			n, err := fmt.Sscanf(in, "%d-%d", &start, &end)
			if n != 2 || err != nil {
				logrus.Errorf("Failed to parse range string with err %v", err)
				return
			}
		}
		if err := portallocator.Get().SetPortRange(start, end); err != nil {
			logrus.Errorf("Failed to set port range with err %v", err)
		}
	}
}

// OptionNetworkControlPlaneMTU function returns an option setter for control plane MTU
func OptionNetworkControlPlaneMTU(exp int) Option {
	return func(c *Config) {
		logrus.Debugf("Network Control Plane MTU: %d", exp)
		if exp < warningThNetworkControlPlaneMTU {
			logrus.Warnf("Received a MTU of %d, this value is very low, the network control plane can misbehave,"+
				" defaulting to minimum value (%d)", exp, minimumNetworkControlPlaneMTU)
			if exp < minimumNetworkControlPlaneMTU {
				exp = minimumNetworkControlPlaneMTU
			}
		}
		c.Daemon.NetworkControlPlaneMTU = exp
	}
}

// ProcessOptions processes options and stores it in config
func (c *Config) ProcessOptions(options ...Option) {
	for _, opt := range options {
		if opt != nil {
			opt(c)
		}
	}
}

// IsValidName validates configuration objects supported by libnetwork
func IsValidName(name string) bool {
	return strings.TrimSpace(name) != ""
}

// OptionLocalKVProvider function returns an option setter for kvstore provider
func OptionLocalKVProvider(provider string) Option {
	return func(c *Config) {
		logrus.Debugf("Option OptionLocalKVProvider: %s", provider)
		if _, ok := c.Scopes[datastore.LocalScope]; !ok {
			c.Scopes[datastore.LocalScope] = &datastore.ScopeCfg{}
		}
		c.Scopes[datastore.LocalScope].Client.Provider = strings.TrimSpace(provider)
	}
}

// OptionLocalKVProviderURL function returns an option setter for kvstore url
func OptionLocalKVProviderURL(url string) Option {
	return func(c *Config) {
		logrus.Debugf("Option OptionLocalKVProviderURL: %s", url)
		if _, ok := c.Scopes[datastore.LocalScope]; !ok {
			c.Scopes[datastore.LocalScope] = &datastore.ScopeCfg{}
		}
		c.Scopes[datastore.LocalScope].Client.Address = strings.TrimSpace(url)
	}
}

// OptionLocalKVProviderConfig function returns an option setter for kvstore config
func OptionLocalKVProviderConfig(config *store.Config) Option {
	return func(c *Config) {
		logrus.Debugf("Option OptionLocalKVProviderConfig: %v", config)
		if _, ok := c.Scopes[datastore.LocalScope]; !ok {
			c.Scopes[datastore.LocalScope] = &datastore.ScopeCfg{}
		}
		c.Scopes[datastore.LocalScope].Client.Config = config
	}
}

// OptionActiveSandboxes function returns an option setter for passing the sandboxes
// which were active during previous daemon life
func OptionActiveSandboxes(sandboxes map[string]interface{}) Option {
	return func(c *Config) {
		c.ActiveSandboxes = sandboxes
	}
}
