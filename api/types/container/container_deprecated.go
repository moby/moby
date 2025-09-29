package container

import (
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/storage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerJSONBase contains response of Engine API GET "/containers/{name:.*}/json"
// for API version 1.18 and older.
//
// TODO(thaJeztah): combine ContainerJSONBase and InspectResponse into a single struct.
// The split between ContainerJSONBase (ContainerJSONBase) and InspectResponse (InspectResponse)
// was done in commit 6deaa58ba5f051039643cedceee97c8695e2af74 (https://github.com/moby/moby/pull/13675).
// ContainerJSONBase contained all fields for API < 1.19, and InspectResponse
// held fields that were added in API 1.19 and up. Given that the minimum
// supported API version is now 1.24, we no longer use the separate type.
type ContainerJSONBase struct {
	ID              string `json:"Id"`
	Created         string
	Path            string
	Args            []string
	State           *State
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *HostConfig
	GraphDriver     storage.DriverData
	SizeRw          *int64 `json:",omitempty"`
	SizeRootFs      *int64 `json:",omitempty"`
}

// InspectResponse is the response for the GET "/containers/{name:.*}/json"
// endpoint.
type InspectResponse struct {
	*ContainerJSONBase
	Mounts          []MountPoint
	Config          *Config
	NetworkSettings *NetworkSettings
	// ImageManifestDescriptor is the descriptor of a platform-specific manifest of the image used to create the container.
	ImageManifestDescriptor *ocispec.Descriptor `json:"ImageManifestDescriptor,omitempty"`
}

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase
	DefaultNetworkSettings
	Networks map[string]*network.EndpointSettings
}

// NetworkSettingsBase holds networking state for a container when inspecting it.
//
// Deprecated: Most fields in NetworkSettingsBase are deprecated. Fields which aren't deprecated will move to
// NetworkSettings in v29.0, and this struct will be removed.
type NetworkSettingsBase struct {
	Bridge     string      // Deprecated: This field is only set when the daemon is started with the --bridge flag specified.
	SandboxID  string      // SandboxID uniquely represents a container's network stack
	SandboxKey string      // SandboxKey identifies the sandbox
	Ports      nat.PortMap // Ports is a collection of PortBinding indexed by Port

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

// DefaultNetworkSettings holds the networking state for the default bridge, if the container is connected to that
// network.
//
// Deprecated: this struct is deprecated since Docker v1.11 and will be removed in v29. You should look for the default
// network in NetworkSettings.Networks instead.
type DefaultNetworkSettings struct {
	// EndpointID uniquely represents a service endpoint in a Sandbox
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	EndpointID string
	// Gateway holds the gateway address for the network
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	Gateway string
	// GlobalIPv6Address holds network's global IPv6 address
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	GlobalIPv6Address string
	// GlobalIPv6PrefixLen represents mask length of network's global IPv6 address
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	GlobalIPv6PrefixLen int
	// IPAddress holds the IPv4 address for the network
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	IPAddress string
	// IPPrefixLen represents mask length of network's IPv4 address
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	IPPrefixLen int
	// IPv6Gateway holds gateway address specific for IPv6
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	IPv6Gateway string
	// MacAddress holds the MAC address for the network
	//
	// Deprecated: This field will be removed in v29. You should look for the default network in NetworkSettings.Networks instead.
	MacAddress string
	Networks   map[string]*network.EndpointSettings
}
