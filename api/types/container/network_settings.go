package container

import (
	"github.com/moby/moby/api/types/network"
)

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase
	Networks map[string]*network.EndpointSettings
}

// NetworkSettingsBase holds networking state for a container when inspecting it.
//
// Deprecated: Most fields in NetworkSettingsBase are deprecated. Fields which aren't deprecated will move to
// NetworkSettings in v29.0, and this struct will be removed.
type NetworkSettingsBase struct {
	Bridge     string  // Deprecated: This field is only set when the daemon is started with the --bridge flag specified.
	SandboxID  string  // SandboxID uniquely represents a container's network stack
	SandboxKey string  // SandboxKey identifies the sandbox
	Ports      PortMap // Ports is a collection of PortBinding indexed by Port

	// HairpinMode specifies if hairpin NAT should be enabled on the virtual interface
	//
	// Deprecated: This field is never set and will be removed in a future release.
	HairpinMode bool
	// LinkLocalIPv6Address is an IPv6 unicast address using the link-local prefix
	//
	// Deprecated: This field is never set and will be removed in a future release.
	LinkLocalIPv6Address string
	// LinkLocalIPv6PrefixLen is the prefix length of an IPv6 unicast address
	//
	// Deprecated: This field is never set and will be removed in a future release.
	LinkLocalIPv6PrefixLen int
	SecondaryIPAddresses   []network.Address // Deprecated: This field is never set and will be removed in a future release.
	SecondaryIPv6Addresses []network.Address // Deprecated: This field is never set and will be removed in a future release.
}

// NetworkSettingsSummary provides a summary of container's networks
// in /containers/json
type NetworkSettingsSummary struct {
	Networks map[string]*network.EndpointSettings
}
