package config

import "github.com/docker/docker/libnetwork/osl"

// optionExecRoot on Linux sets both the controller's ExecRoot and osl.basePath.
func optionExecRoot(execRoot string) Option {
	return func(c *Config) {
		c.ExecRoot = execRoot
		osl.SetBasePath(execRoot)
	}
}
