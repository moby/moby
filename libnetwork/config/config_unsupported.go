//go:build !linux && !freebsd

package config

// optionExecRoot is a no-op on non-unix platforms.
func optionExecRoot(execRoot string) Option {
	return func(*Config) {}
}
