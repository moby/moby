package config

import (
	"strings"

	"github.com/BurntSushi/toml"
)

// Config encapsulates configurations of various Libnetwork components
type Config struct {
	Daemon    DaemonCfg
	Cluster   ClusterCfg
	Datastore DatastoreCfg
}

// DaemonCfg represents libnetwork core configuration
type DaemonCfg struct {
	Debug          bool
	DefaultNetwork string
	DefaultDriver  string
}

// ClusterCfg represents cluster configuration
type ClusterCfg struct {
	Discovery string
	Address   string
	Heartbeat uint64
}

// DatastoreCfg represents Datastore configuration.
type DatastoreCfg struct {
	Embedded bool
	Client   DatastoreClientCfg
}

// DatastoreClientCfg represents Datastore Client-only mode configuration
type DatastoreClientCfg struct {
	Provider string
	Address  string
}

// ParseConfig parses the libnetwork configuration file
func ParseConfig(tomlCfgFile string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(tomlCfgFile, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Option is a option setter function type used to pass varios configurations
// to the controller
type Option func(c *Config)

// OptionDefaultNetwork function returns an option setter for a default network
func OptionDefaultNetwork(dn string) Option {
	return func(c *Config) {
		c.Daemon.DefaultNetwork = strings.TrimSpace(dn)
	}
}

// OptionDefaultDriver function returns an option setter for default driver
func OptionDefaultDriver(dd string) Option {
	return func(c *Config) {
		c.Daemon.DefaultDriver = strings.TrimSpace(dd)
	}
}

// OptionKVProvider function returns an option setter for kvstore provider
func OptionKVProvider(provider string) Option {
	return func(c *Config) {
		c.Datastore.Client.Provider = strings.TrimSpace(provider)
	}
}

// OptionKVProviderURL function returns an option setter for kvstore url
func OptionKVProviderURL(url string) Option {
	return func(c *Config) {
		c.Datastore.Client.Address = strings.TrimSpace(url)
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
