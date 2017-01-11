// +build solaris linux freebsd

package daemon

import (
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"
	"github.com/spf13/pflag"
)

// CommonUnixConfig defines configuration of a docker daemon that is
// common across Unix platforms.
type CommonUnixConfig struct {
	ExecRoot       string                   `json:"exec-root,omitempty"`
	ContainerdAddr string                   `json:"containerd,omitempty"`
	Runtimes       map[string]types.Runtime `json:"runtimes,omitempty"`
	DefaultRuntime string                   `json:"default-runtime,omitempty"`
}

type commonUnixBridgeConfig struct {
	DefaultIP                   net.IP `json:"ip,omitempty"`
	IP                          string `json:"bip,omitempty"`
	DefaultGatewayIPv4          net.IP `json:"default-gateway,omitempty"`
	DefaultGatewayIPv6          net.IP `json:"default-gateway-v6,omitempty"`
	InterContainerCommunication bool   `json:"icc,omitempty"`
}

// InstallCommonUnixFlags adds command-line options to the top-level flag parser for
// the current process that are common across Unix platforms.
func (config *Config) InstallCommonUnixFlags(flags *pflag.FlagSet) {
	config.Runtimes = make(map[string]types.Runtime)

	flags.StringVarP(&config.SocketGroup, "group", "G", "docker", "Group for the unix socket")
	flags.StringVar(&config.bridgeConfig.IP, "bip", "", "Specify network bridge IP")
	flags.StringVarP(&config.bridgeConfig.Iface, "bridge", "b", "", "Attach containers to a network bridge")
	flags.StringVar(&config.bridgeConfig.FixedCIDR, "fixed-cidr", "", "IPv4 subnet for fixed IPs")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultGatewayIPv4, ""), "default-gateway", "Container default gateway IPv4 address")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultGatewayIPv6, ""), "default-gateway-v6", "Container default gateway IPv6 address")
	flags.BoolVar(&config.bridgeConfig.InterContainerCommunication, "icc", true, "Enable inter-container communication")
	flags.Var(opts.NewIPOpt(&config.bridgeConfig.DefaultIP, "0.0.0.0"), "ip", "Default IP when binding container ports")
	flags.Var(opts.NewNamedRuntimeOpt("runtimes", &config.Runtimes, stockRuntimeName), "add-runtime", "Register an additional OCI compatible runtime")
	flags.StringVar(&config.DefaultRuntime, "default-runtime", stockRuntimeName, "Default OCI runtime for containers")

}

// GetRuntime returns the runtime path and arguments for a given
// runtime name
func (config *Config) GetRuntime(name string) *types.Runtime {
	config.reloadLock.Lock()
	defer config.reloadLock.Unlock()
	if rt, ok := config.Runtimes[name]; ok {
		return &rt
	}
	return nil
}

// GetDefaultRuntimeName returns the current default runtime
func (config *Config) GetDefaultRuntimeName() string {
	config.reloadLock.Lock()
	rt := config.DefaultRuntime
	config.reloadLock.Unlock()

	return rt
}

// GetAllRuntimes returns a copy of the runtimes map
func (config *Config) GetAllRuntimes() map[string]types.Runtime {
	config.reloadLock.Lock()
	rts := config.Runtimes
	config.reloadLock.Unlock()
	return rts
}

// GetExecRoot returns the user configured Exec-root
func (config *Config) GetExecRoot() string {
	return config.ExecRoot
}

// GetInitPath returns the configure docker-init path
func (config *Config) GetInitPath() string {
	config.reloadLock.Lock()
	defer config.reloadLock.Unlock()
	if config.InitPath != "" {
		return config.InitPath
	}
	return DefaultInitBinary
}
