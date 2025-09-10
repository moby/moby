package network

const (
	// NetworkDefault is a platform-independent alias to choose the platform-specific default network stack.
	NetworkDefault = "default"
	// NetworkHost is the name of the predefined network used when the NetworkMode host is selected (only available on Linux)
	NetworkHost = "host"
	// NetworkNone is the name of the predefined network used when the NetworkMode none is selected (available on both Linux and Windows)
	NetworkNone = "none"
	// NetworkBridge is the name of the default network on Linux
	NetworkBridge = "bridge"
	// NetworkNat is the name of the default network on Windows
	NetworkNat = "nat"
)

// CreateRequest is the request message sent to the server for network create call.
type CreateRequest struct {
	Name       string            // Name is the requested name of the network.
	Driver     string            // Driver is the driver-name used to create the network (e.g. `bridge`, `overlay`)
	Scope      string            // Scope describes the level at which the network exists (e.g. `swarm` for cluster-wide or `local` for machine level).
	EnableIPv4 *bool             `json:",omitempty"` // EnableIPv4 represents whether to enable IPv4.
	EnableIPv6 *bool             `json:",omitempty"` // EnableIPv6 represents whether to enable IPv6.
	IPAM       *IPAM             // IPAM is the network's IP Address Management.
	Internal   bool              // Internal represents if the network is used internal only.
	Attachable bool              // Attachable represents if the global scope is manually attachable by regular containers from workers in swarm mode.
	Ingress    bool              // Ingress indicates the network is providing the routing-mesh for the swarm cluster.
	ConfigOnly bool              // ConfigOnly creates a config-only network. Config-only networks are place-holder networks for network configurations to be used by other networks. ConfigOnly networks cannot be used directly to run containers or services.
	ConfigFrom *ConfigReference  // ConfigFrom specifies the source which will provide the configuration for this network. The specified network must be a config-only network; see [CreateOptions.ConfigOnly].
	Options    map[string]string // Options specifies the network-specific options to use for when creating the network.
	Labels     map[string]string // Labels holds metadata specific to the network being created.
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings // Endpoint configs for each connecting network
}

// PruneReport contains the response for Engine API:
// POST "/networks/prune"
type PruneReport struct {
	NetworksDeleted []string
}
