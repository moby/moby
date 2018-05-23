package network // import "github.com/docker/docker/api/types/network"

import (
	"time"
)

// Address represents an IP address
type Address struct {
	Addr      string
	PrefixLen int
}

// IPAM represents IP Address Management
type IPAM struct {
	Driver  string
	Options map[string]string //Per network IPAM driver options
	Config  []IPAMConfig
}

// IPAMConfig represents IPAM configurations
type IPAMConfig struct {
	Subnet     string            `json:",omitempty"`
	IPRange    string            `json:",omitempty"`
	Gateway    string            `json:",omitempty"`
	AuxAddress map[string]string `json:"AuxiliaryAddresses,omitempty"`
}

// EndpointIPAMConfig represents IPAM configurations for the endpoint
type EndpointIPAMConfig struct {
	IPv4Address  string   `json:",omitempty"`
	IPv6Address  string   `json:",omitempty"`
	LinkLocalIPs []string `json:",omitempty"`
}

// Copy makes a copy of the endpoint ipam config
func (cfg *EndpointIPAMConfig) Copy() *EndpointIPAMConfig {
	cfgCopy := *cfg
	cfgCopy.LinkLocalIPs = make([]string, 0, len(cfg.LinkLocalIPs))
	cfgCopy.LinkLocalIPs = append(cfgCopy.LinkLocalIPs, cfg.LinkLocalIPs...)
	return &cfgCopy
}

// PeerInfo represents one peer of an overlay network
type PeerInfo struct {
	Name string
	IP   string
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	IPAMConfig *EndpointIPAMConfig
	Links      []string
	Aliases    []string
	// Operational data
	NetworkID           string
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	MacAddress          string
	DriverOpts          map[string]string
}

// Task carries the information about one backend task
type Task struct {
	Name       string
	EndpointID string
	EndpointIP string
	Info       map[string]string
}

// ServiceInfo represents service parameters with the list of service's tasks
type ServiceInfo struct {
	VIP          string
	Ports        []string
	LocalLBIndex int
	Tasks        []Task
}

// Copy makes a deep copy of `EndpointSettings`
func (es *EndpointSettings) Copy() *EndpointSettings {
	epCopy := *es
	if es.IPAMConfig != nil {
		epCopy.IPAMConfig = es.IPAMConfig.Copy()
	}

	if es.Links != nil {
		links := make([]string, 0, len(es.Links))
		epCopy.Links = append(links, es.Links...)
	}

	if es.Aliases != nil {
		aliases := make([]string, 0, len(es.Aliases))
		epCopy.Aliases = append(aliases, es.Aliases...)
	}
	return &epCopy
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings // Endpoint configs for each connecting network
}

// ConfigReference specifies the source which provides a network's configuration
type ConfigReference struct {
	Network string
}

// Resource is the body of the "get network" http response message
type Resource struct {
	Name       string                      // Name is the requested name of the network
	ID         string                      `json:"Id"` // ID uniquely identifies a network on a single machine
	Created    time.Time                   // Created is the time the network created
	Scope      string                      // Scope describes the level at which the network exists (e.g. `swarm` for cluster-wide or `local` for machine level)
	Driver     string                      // Driver is the Driver name used to create the network (e.g. `bridge`, `overlay`)
	EnableIPv6 bool                        // EnableIPv6 represents whether to enable IPv6
	IPAM       IPAM                        // IPAM is the network's IP Address Management
	Internal   bool                        // Internal represents if the network is used internal only
	Attachable bool                        // Attachable represents if the global scope is manually attachable by regular containers from workers in swarm mode.
	Ingress    bool                        // Ingress indicates the network is providing the routing-mesh for the swarm cluster.
	ConfigFrom ConfigReference             // ConfigFrom specifies the source which will provide the configuration for this network.
	ConfigOnly bool                        // ConfigOnly networks are place-holder networks for network configurations to be used by other networks. ConfigOnly networks cannot be used directly to run containers or services.
	Containers map[string]EndpointResource // Containers contains endpoints belonging to the network
	Options    map[string]string           // Options holds the network specific options to use for when creating the network
	Labels     map[string]string           // Labels holds metadata specific to the network being created
	Peers      []PeerInfo                  `json:",omitempty"` // List of peer nodes for an overlay network
	Services   map[string]ServiceInfo      `json:",omitempty"`
}

// EndpointResource contains network resources allocated and used for a container in a network
type EndpointResource struct {
	Name        string
	EndpointID  string
	MacAddress  string
	IPv4Address string
	IPv6Address string
}

// NetworkCreate is the expected body of the "create network" http request message
type NetworkCreate struct {
	// Check for networks with duplicate names.
	// Network is primarily keyed based on a random ID and not on the name.
	// Network name is strictly a user-friendly alias to the network
	// which is uniquely identified using ID.
	// And there is no guaranteed way to check for duplicates.
	// Option CheckDuplicate is there to provide a best effort checking of any networks
	// which has the same name but it is not guaranteed to catch all name collisions.
	CheckDuplicate bool
	Driver         string
	Scope          string
	EnableIPv6     bool
	IPAM           *IPAM
	Internal       bool
	Attachable     bool
	Ingress        bool
	ConfigOnly     bool
	ConfigFrom     *ConfigReference
	Options        map[string]string
	Labels         map[string]string
}

// CreateRequest is the request message sent to the server for network create call.
type CreateRequest struct {
	NetworkCreate
	Name string
}

// CreateResponse is the response message sent by the server for network create call
type CreateResponse struct {
	ID      string `json:"Id"`
	Warning string
}

// Connect represents the data to be used to connect a container to the network
type Connect struct {
	Container      string
	EndpointConfig *EndpointSettings `json:",omitempty"`
}

// Disconnect represents the data to be used to disconnect a container from the network
type Disconnect struct {
	Container string
	Force     bool
}
