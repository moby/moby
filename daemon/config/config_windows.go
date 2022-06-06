package config // import "github.com/docker/docker/daemon/config"

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
)

const (
	// This is used by the `default-runtime` flag in dockerd as the default value.
	// On windows we'd prefer to keep this empty so the value is auto-detected based on other options.
	StockRuntimeName = ""
)

// BridgeConfig stores all the bridge driver specific
// configuration.
type BridgeConfig struct {
	commonBridgeConfig
}

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// Fields below here are platform specific. (There are none presently
	// for the Windows daemon.)
}

// GetRuntime returns the runtime path and arguments for a given
// runtime name
func (conf *Config) GetRuntime(name string) *types.Runtime {
	return nil
}

// GetAllRuntimes returns a copy of the runtimes map
func (conf *Config) GetAllRuntimes() map[string]types.Runtime {
	return map[string]types.Runtime{}
}

// GetExecRoot returns the user configured Exec-root
func (conf *Config) GetExecRoot() string {
	return ""
}

// GetInitPath returns the configured docker-init path
func (conf *Config) GetInitPath() string {
	return ""
}

// IsSwarmCompatible defines if swarm mode can be enabled in this config
func (conf *Config) IsSwarmCompatible() error {
	return nil
}

// ValidatePlatformConfig checks if any platform-specific configuration settings are invalid.
func (conf *Config) ValidatePlatformConfig() error {
	return nil
}

// IsRootless returns conf.Rootless on Linux but false on Windows
func (conf *Config) IsRootless() bool {
	return false
}

func setPlatformDefaults(cfg *Config) error {
	cfg.Root = filepath.Join(os.Getenv("programdata"), "docker")
	cfg.ExecRoot = filepath.Join(os.Getenv("programdata"), "docker", "exec-root")
	cfg.Pidfile = filepath.Join(cfg.Root, "docker.pid")
	return nil
}
