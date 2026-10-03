package config

// PlatformConfig defines platform-specific configuration.
type PlatformConfig struct{}

func defaultPlatformConfig() PlatformConfig {
	return PlatformConfig{}
}
