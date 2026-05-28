package config

import (
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/libnetwork/osl"
)

// PlatformConfig defines platform-specific configuration.
type PlatformConfig struct {
	BridgeConfig bridge.Configuration
}

// OptionBridgeConfig returns an option setter for bridge driver config.
func OptionBridgeConfig(config bridge.Configuration) Option {
	return func(c *Config) {
		c.BridgeConfig = config
	}
}

// optionExecRoot on Linux sets both the controller's ExecRoot and osl.basePath.
func optionExecRoot(execRoot string) Option {
	return func(c *Config) {
		c.ExecRoot = execRoot
		osl.SetBasePath(execRoot)
	}
}
