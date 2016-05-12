// +build experimental

package container

import "github.com/spf13/pflag"

func addExperimentalStartFlags(flags *pflag.FlagSet, opts *startOptions) {
	flags.StringVar(&opts.checkpoint, "checkpoint", "", "Restore from this checkpoint")
}
