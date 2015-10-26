package network

import "github.com/docker/docker/pkg/nat"

// Address represents an IP address
type Address struct {
	Addr      string
	PrefixLen int
}

// IPAM represents IP Address Management
type IPAM struct {
	Driver string       `json:"driver"`
	Config []IPAMConfig `json:"config"`
}

// IPAMConfig represents IPAM configurations
type IPAMConfig struct {
	Subnet     string            `json:"subnet,omitempty"`
	IPRange    string            `json:"ip_range,omitempty"`
	Gateway    string            `json:"gateway,omitempty"`
	AuxAddress map[string]string `json:"auxiliary_address,omitempty"`
}

// Settings stores configuration details about the daemon network config
// TODO Windows. Many of these fields can be factored out.,
type Settings struct {
	Bridge                 string
	EndpointID             string // this is for backward compatibility
	SandboxID              string
	Gateway                string // this is for backward compatibility
	GlobalIPv6Address      string // this is for backward compatibility
	GlobalIPv6PrefixLen    int    // this is for backward compatibility
	HairpinMode            bool
	IPAddress              string // this is for backward compatibility
	IPPrefixLen            int    // this is for backward compatibility
	IPv6Gateway            string // this is for backward compatibility
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	MacAddress             string // this is for backward compatibility
	Networks               map[string]*EndpointSettings
	Ports                  nat.PortMap
	SandboxKey             string
	SecondaryIPAddresses   []Address
	SecondaryIPv6Addresses []Address
	IsAnonymousEndpoint    bool
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	MacAddress          string
}
