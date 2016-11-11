package daemon

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/spf13/pflag"
)

var (
	defaultPidFile string
	defaultGraph   = filepath.Join(os.Getenv("programdata"), "docker")
)

// bridgeConfig stores all the bridge driver specific
// configuration.
type bridgeConfig struct {
	commonBridgeConfig
}

// Config defines the configuration of a docker daemon.
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker daemon -e windows`
type Config struct {
	CommonConfig

	// Fields below here are platform specific. (There are none presently
	// for the Windows daemon.)
}

// InstallFlags adds flags to the pflag.FlagSet to configure the daemon
func (config *Config) InstallFlags(flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	config.InstallCommonFlags(flags)

	// Then platform-specific install flags.
	flags.StringVar(&config.bridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.StringVarP(&config.bridgeConfig.Iface, "bridge", "b", "", "Attach containers to a virtual switch")
	flags.StringVarP(&config.SocketGroup, "group", "G", "", "Users or groups that can access the named pipe")
}

// GetRuntime returns the runtime path and arguments for a given
// runtime name
func (config *Config) GetRuntime(name string) *types.Runtime {
	return nil
}

// GetInitPath returns the configure docker-init path
func (config *Config) GetInitPath() string {
	return ""
}

// GetDefaultRuntimeName returns the current default runtime
func (config *Config) GetDefaultRuntimeName() string {
	return stockRuntimeName
}

// GetAllRuntimes returns a copy of the runtimes map
func (config *Config) GetAllRuntimes() map[string]types.Runtime {
	return map[string]types.Runtime{}
}

// GetExecRoot returns the user configured Exec-root
func (config *Config) GetExecRoot() string {
	return ""
}

func (config *Config) isSwarmCompatible() error {
	return nil
}
