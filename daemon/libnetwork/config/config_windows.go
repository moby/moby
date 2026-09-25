package config

// PlatformConfig defines platform-specific configuration.
type PlatformConfig struct{}

// optionExecRoot is a no-op on non-unix platforms.
func optionExecRoot(execRoot string) Option {
	return func(*Config) {}
}
