package config

import (
	resolverconfig "github.com/moby/buildkit/util/resolver/config"
)

// Config provides containerd configuration data for the server
type Config struct {
	Debug bool `toml:"debug"`

	// Root is the path to a directory where buildkit will store persistent data
	Root string `toml:"root"`

	// Entitlements e.g. security.insecure, network.host
	Entitlements []string `toml:"insecure-entitlements"`
	// GRPC configuration settings
	GRPC GRPCConfig `toml:"grpc"`

	Workers struct {
		OCI        OCIConfig        `toml:"oci"`
		Containerd ContainerdConfig `toml:"containerd"`
	} `toml:"worker"`

	Registries map[string]resolverconfig.RegistryConfig `toml:"registry"`

	DNS *DNSConfig `toml:"dns"`

	History *HistoryConfig `toml:"history"`
}

type GRPCConfig struct {
	Address      []string `toml:"address"`
	DebugAddress string   `toml:"debugAddress"`
	UID          *int     `toml:"uid"`
	GID          *int     `toml:"gid"`

	TLS TLSConfig `toml:"tls"`
	// MaxRecvMsgSize int    `toml:"max_recv_message_size"`
	// MaxSendMsgSize int    `toml:"max_send_message_size"`
}

type TLSConfig struct {
	Cert string `toml:"cert"`
	Key  string `toml:"key"`
	CA   string `toml:"ca"`
}

type GCConfig struct {
	GC            *bool      `toml:"gc"`
	GCKeepStorage int64      `toml:"gckeepstorage"`
	GCPolicy      []GCPolicy `toml:"gcpolicy"`
}

type NetworkConfig struct {
	Mode          string `toml:"networkMode"`
	CNIConfigPath string `toml:"cniConfigPath"`
	CNIBinaryPath string `toml:"cniBinaryPath"`
	CNIPoolSize   int    `toml:"cniPoolSize"`
}

type OCIConfig struct {
	Enabled          *bool             `toml:"enabled"`
	Labels           map[string]string `toml:"labels"`
	Platforms        []string          `toml:"platforms"`
	Snapshotter      string            `toml:"snapshotter"`
	Rootless         bool              `toml:"rootless"`
	NoProcessSandbox bool              `toml:"noProcessSandbox"`
	GCConfig
	NetworkConfig
	// UserRemapUnsupported is unsupported key for testing. The feature is
	// incomplete and the intention is to make it default without config.
	UserRemapUnsupported string `toml:"userRemapUnsupported"`
	// For use in storing the OCI worker binary name that will replace buildkit-runc
	Binary               string `toml:"binary"`
	ProxySnapshotterPath string `toml:"proxySnapshotterPath"`
	DefaultCgroupParent  string `toml:"defaultCgroupParent"`

	// StargzSnapshotterConfig is configuration for stargz snapshotter.
	// We use a generic map[string]interface{} in order to remove the dependency
	// on stargz snapshotter's config pkg from our config.
	StargzSnapshotterConfig map[string]interface{} `toml:"stargzSnapshotter"`

	// ApparmorProfile is the name of the apparmor profile that should be used to constrain build containers.
	// The profile should already be loaded (by a higher level system) before creating a worker.
	ApparmorProfile string `toml:"apparmor-profile"`

	// SELinux enables applying SELinux labels.
	SELinux bool `toml:"selinux"`

	// MaxParallelism is the maximum number of parallel build steps that can be run at the same time.
	MaxParallelism int `toml:"max-parallelism"`
}

type ContainerdConfig struct {
	Address   string            `toml:"address"`
	Enabled   *bool             `toml:"enabled"`
	Labels    map[string]string `toml:"labels"`
	Platforms []string          `toml:"platforms"`
	Namespace string            `toml:"namespace"`
	GCConfig
	NetworkConfig
	Snapshotter string `toml:"snapshotter"`

	// ApparmorProfile is the name of the apparmor profile that should be used to constrain build containers.
	// The profile should already be loaded (by a higher level system) before creating a worker.
	ApparmorProfile string `toml:"apparmor-profile"`

	// SELinux enables applying SELinux labels.
	SELinux bool `toml:"selinux"`

	MaxParallelism int `toml:"max-parallelism"`

	Rootless bool `toml:"rootless"`
}

type GCPolicy struct {
	All          bool     `toml:"all"`
	KeepBytes    int64    `toml:"keepBytes"`
	KeepDuration int64    `toml:"keepDuration"`
	Filters      []string `toml:"filters"`
}

type DNSConfig struct {
	Nameservers   []string `toml:"nameservers"`
	Options       []string `toml:"options"`
	SearchDomains []string `toml:"searchDomains"`
}

type HistoryConfig struct {
	MaxAge     int64 `toml:"maxAge"`
	MaxEntries int64 `toml:"maxEntries"`
}
