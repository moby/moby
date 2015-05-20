// +build experimental

package runconfig

import flag "github.com/docker/docker/pkg/mflag"

type experimentalFlags struct {
	flags map[string]interface{}
}

func attachExperimentalFlags(cmd *flag.FlagSet) *experimentalFlags {
	flags := make(map[string]interface{})
	flags["volume-driver"] = cmd.String([]string{"-volume-driver"}, "", "Optional volume driver for the container")
	return &experimentalFlags{flags: flags}
}

func applyExperimentalFlags(exp *experimentalFlags, config *Config, hostConfig *HostConfig) {
	config.VolumeDriver = *(exp.flags["volume-driver"]).(*string)
}
