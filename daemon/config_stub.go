// +build !experimental

package daemon

import flag "github.com/docker/docker/pkg/mflag"

func (config *Config) attachExperimentalFlags(cmd *flag.FlagSet, usageFn func(string) string) {
}
