package daemon

import (
	"github.com/spf13/pflag"
)

var (
	defaultPidFile = "/var/run/docker.pid"
	defaultGraph   = "/var/lib/docker"
	defaultExec    = "zones"
)

// Config defines the configuration of a docker daemon.
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
type Config struct {
	CommonConfig

	// Fields below here are platform specific.
	ExecRoot string `json:"exec-root,omitempty"`
}

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig struct {
	commonBridgeConfig
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
func (config *Config) InstallFlags(flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags(flags)

	// Then platform-specific install flags
	config.attachExperimentalFlags(flags)
}

// GetExecRoot returns the user configured Exec-root
func (config *Config) GetExecRoot() string {
	return config.ExecRoot
}
func (config *Config) isSwarmCompatible() error {
	return nil
}
