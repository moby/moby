package main

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/daemon/config"
	"github.com/spf13/pflag"
)

func getDefaultPidFile() (string, error) {
	return "", nil
}

func getDefaultDataRoot() (string, error) {
	return filepath.Join(os.Getenv("programdata"), "docker"), nil
}

func getDefaultExecRoot() (string, error) {
	return filepath.Join(os.Getenv("programdata"), "docker", "exec-root"), nil
}

// installConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) error {
	// First handle install flags which are consistent cross-platform
	if err := installCommonConfigFlags(conf, flags); err != nil {
		return err
	}

	// Then platform-specific install flags.
	flags.StringVar(&conf.BridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.StringVarP(&conf.BridgeConfig.Iface, "bridge", "b", "", "Attach containers to a virtual switch")
	flags.StringVarP(&conf.SocketGroup, "group", "G", "", "Users or groups that can access the named pipe")
	return nil
}

// configureCertsDir configures registry.CertsDir() depending on if the daemon
// is running in rootless mode or not. On Windows, it is a no-op.
func configureCertsDir() {}
