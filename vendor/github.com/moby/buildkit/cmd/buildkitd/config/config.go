package config

import "github.com/BurntSushi/toml"

// Config provides containerd configuration data for the server
type Config struct {
	Debug bool `toml:"debug"`

	// Root is the path to a directory where buildkit will store persistent data
	Root string `toml:"root"`

	//Entitlements e.g. security.insecure, network.host
	Entitlements []string `toml:"insecure-entitlements"`
	// GRPC configuration settings
	GRPC GRPCConfig `toml:"grpc"`

	Workers struct {
		OCI        OCIConfig        `toml:"oci"`
		Containerd ContainerdConfig `toml:"containerd"`
	} `toml:"worker"`

	Registries map[string]RegistryConfig `toml:"registry"`

	DNS *DNSConfig `toml:"dns"`
}

type GRPCConfig struct {
	Address      []string `toml:"address"`
	DebugAddress string   `toml:"debugAddress"`
	UID          int      `toml:"uid"`
	GID          int      `toml:"gid"`

	TLS TLSConfig `toml:"tls"`
	// MaxRecvMsgSize int    `toml:"max_recv_message_size"`
	// MaxSendMsgSize int    `toml:"max_send_message_size"`
}

type RegistryConfig struct {
	Mirrors      []string     `toml:"mirrors"`
	PlainHTTP    *bool        `toml:"http"`
	Insecure     *bool        `toml:"insecure"`
	RootCAs      []string     `toml:"ca"`
	KeyPairs     []TLSKeyPair `toml:"keypair"`
	TLSConfigDir []string     `toml:"tlsconfigdir"`
}

type TLSKeyPair struct {
	Key         string `toml:"key"`
	Certificate string `toml:"cert"`
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

	// StargzSnapshotterConfig is configuration for stargz snapshotter.
	// Decoding this is delayed in order to remove the dependency from this
	// config pkg to stargz snapshotter's config pkg.
	StargzSnapshotterConfig toml.Primitive `toml:"stargzSnapshotter"`

	// ApparmorProfile is the name of the apparmor profile that should be used to constrain build containers.
	// The profile should already be loaded (by a higher level system) before creating a worker.
	ApparmorProfile string `toml:"apparmor-profile"`
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
