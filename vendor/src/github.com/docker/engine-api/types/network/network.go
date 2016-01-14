package network

// Address represents an IP address
type Address struct {
	Addr      string `json:",omitempty"`
	PrefixLen int    `json:",omitempty"`
}

// IPAM represents IP Address Management
type IPAM struct {
	Driver  string            `json:",omitempty"`
	Options map[string]string `json:",omitempty"` //Per network IPAM driver options
	Config  []IPAMConfig      `json:",omitempty"`
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
	IPv4Address string `json:",omitempty"`
	IPv6Address string `json:",omitempty"`
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	IPAMConfig *EndpointIPAMConfig `json:",omitempty"`
	Links      []string            `json:",omitempty"`
	Aliases    []string            `json:",omitempty"`
	// Operational data
	NetworkID           string `json:",omitempty"`
	EndpointID          string `json:",omitempty"`
	Gateway             string `json:",omitempty"`
	IPAddress           string `json:",omitempty"`
	IPPrefixLen         int    `json:",omitempty"`
	IPv6Gateway         string `json:",omitempty"`
	GlobalIPv6Address   string `json:",omitempty"`
	GlobalIPv6PrefixLen int    `json:",omitempty"`
	MacAddress          string `json:",omitempty"`
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networink configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings `json:",omitempty"` // Endpoint configs for each conencting network
}
