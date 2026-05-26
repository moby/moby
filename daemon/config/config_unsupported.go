//go:build !linux && !windows

package config

func setPlatformDefaults(*Config) error  { return nil }
func validatePlatformConfig(*Config) error { return nil }
