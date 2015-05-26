package config

import "github.com/BurntSushi/toml"

// Config encapsulates configurations of various Libnetwork components
type Config struct {
	Daemon    DaemonCfg
	Cluster   ClusterCfg
	Datastore DatastoreCfg
}

// DaemonCfg represents libnetwork core configuration
type DaemonCfg struct {
	Debug bool
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
