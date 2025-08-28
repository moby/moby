package command

import (
	"github.com/moby/moby/v2/daemon/config"
	"github.com/spf13/pflag"
)

// installConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	installCommonConfigFlags(conf, flags)

	// Then platform-specific install flags.
	flags.StringVar(&conf.BridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.StringVarP(&conf.BridgeConfig.Iface, "bridge", "b", "", "Attach containers to a virtual switch")
	flags.StringVarP(&conf.SocketGroup, "group", "G", "", "Users or groups that can access the named pipe")
}

// configureCertsDir configures registry.CertsDir() depending on if the daemon
// is running in rootless mode or not. On Windows, it is a no-op.
func configureCertsDir() {}
