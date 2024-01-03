package config // import "github.com/docker/docker/daemon/config"

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/log"
)

const (
	// StockRuntimeName is used by the 'default-runtime' flag in dockerd as the
	// default value. On Windows keep this empty so the value is auto-detected
	// based on other options.
	StockRuntimeName = ""

	// minAPIVersion represents Minimum REST API version supported
	// Technically the first daemon API version released on Windows is v1.25 in
	// engine version 1.13. However, some clients are explicitly using downlevel
	// APIs (e.g. docker-compose v2.1 file format) and that is just too restrictive.
	// Hence also allowing 1.24 on Windows.
	minAPIVersion string = "1.24"
)

// BridgeConfig is meant to store all the parameters for both the bridge driver and the default bridge network. On
// Windows: 1. "bridge" in this context reference the nat driver and the default nat network; 2. the nat driver has no
// specific parameters, so this struct effectively just stores parameters for the default nat network.
type BridgeConfig struct {
	DefaultBridgeConfig
}

type DefaultBridgeConfig struct {
	commonBridgeConfig

	// MTU is not actually used on Windows, but the --mtu option has always
	// been there on Windows (but ignored).
	MTU int `json:"mtu,omitempty"`
}

// Config defines the configuration of a docker daemon.
// It includes json tags to deserialize configuration from a file
// using the same names that the flags in the command line uses.
type Config struct {
	CommonConfig

	// Fields below here are platform specific. (There are none presently
	// for the Windows daemon.)
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
	if conf.MTU != 0 && conf.MTU != DefaultNetworkMtu {
		log.G(context.TODO()).Warn(`WARNING: MTU for the default network is not configurable on Windows, and this option will be ignored.`)
	}
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
