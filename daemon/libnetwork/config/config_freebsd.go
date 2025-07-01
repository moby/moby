package config

// FIXME(thaJeztah): ExecRoot is only used for Controller.startExternalKeyListener(), but "libnetwork-setkey" is only implemented on Linux.
func optionExecRoot(execRoot string) Option {
	return func(c *Config) {
		c.ExecRoot = execRoot
	}
}
