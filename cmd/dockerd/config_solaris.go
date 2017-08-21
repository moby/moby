package main

import (
	"github.com/docker/docker/daemon/config"
	"github.com/spf13/pflag"
)

// installConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	installCommonConfigFlags(conf, flags)

	// Then install flags common to unix platforms
	installUnixConfigFlags(conf, flags)

	attachExperimentalFlags(conf, flags)
}
