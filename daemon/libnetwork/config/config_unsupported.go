//go:build !linux && !windows

package config

// PlatformConfig defines platform-specific configuration.
type PlatformConfig struct {
	BridgeConfig any
}

// OptionBridgeConfig returns an option setter for bridge driver config.
func OptionBridgeConfig(config any) Option {
	return func(c *Config) {
		c.BridgeConfig = config
	}
}

// optionExecRoot only sets the controller's ExecRoot on unsupported platforms.
func optionExecRoot(execRoot string) Option {
	return func(c *Config) {
		c.ExecRoot = execRoot
	}
}
