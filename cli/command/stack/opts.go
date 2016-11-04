package stack

import "github.com/spf13/pflag"

func addComposefileFlag(opt *string, flags *pflag.FlagSet) {
	flags.StringVar(opt, "compose-file", "", "Path to a Compose file")
}

func addRegistryAuthFlag(opt *bool, flags *pflag.FlagSet) {
	flags.BoolVar(opt, "with-registry-auth", false, "Send registry authentication details to Swarm agents")
}
