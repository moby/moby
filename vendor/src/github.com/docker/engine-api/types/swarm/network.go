package swarm

// Endpoint represents an endpoint.
type Endpoint struct {
	Spec         EndpointSpec        `json:",omitempty"`
	ExposedPorts []PortConfig        `json:",omitempty"`
	VirtualIPs   []EndpointVirtualIP `json:",omitempty"`
}

// EndpointSpec represents the spec of an endpoint.
type EndpointSpec struct {
	Mode         ResolutionMode `json:",omitempty"`
	Ingress      IngressRouting `json:",omitempty"`
	ExposedPorts []PortConfig   `json:",omitempty"`
}

const (
	// ResolutionModeVIP VIP
	ResolutionModeVIP = "VIP"
	// ResolutionModeDNSRR DNSRR
	ResolutionModeDNSRR = "DNSRR"
)

// ResolutionMode represents a resolution mode.
type ResolutionMode string

const (
	// IngressRoutingSWARMPORT SWARMPORT
	IngressRoutingSWARMPORT = "SWARMPORT"
	// IngressRoutingDISABLED DISABLED
	IngressRoutingDISABLED = "DISABLED"
)

// IngressRouting represents an ingress routing.
type IngressRouting string

// PortConfig represents the config of a port.
type PortConfig struct {
	Name      string             `json:",omitempty"`
	Protocol  PortConfigProtocol `json:",omitempty"`
	Port      uint32             `json:",omitempty"`
	SwarmPort uint32             `json:",omitempty"`
}

const (
	// PortConfigProtocolTCP TCP
	PortConfigProtocolTCP = "TCP"
	// PortConfigProtocolUDP UDP
	PortConfigProtocolUDP = "UDP"
)

// PortConfigProtocol represents the protocol of a port.
type PortConfigProtocol string

// EndpointVirtualIP represents the virtual ip of a port.
type EndpointVirtualIP struct {
	NetworkID string `json:",omitempty"`
	Addr      string `json:",omitempty"`
}

// Network represents a network.
type Network struct {
	ID string
	Meta
	Spec        NetworkSpec `json:",omitempty"`
	DriverState Driver      `json:",omitempty"`
	IPAM        IPAMOptions `json:",omitempty"`
}

// NetworkSpec represents the spec of a network.
type NetworkSpec struct {
	Annotations
	DriverConfiguration *Driver      `json:",omitempty"`
	IPv6Enabled         bool         `json:",omitempty"`
	Internal            bool         `json:",omitempty"`
	IPAM                *IPAMOptions `json:",omitempty"`
}

// NetworkAttachmentConfig represents the configuration of a network attachement.
type NetworkAttachmentConfig struct {
	Target string `json:",omitempty"`
}

// NetworkAttachment represents a network attchement.
type NetworkAttachment struct {
	Network   Network  `json:",omitempty"`
	Addresses []string `json:",omitempty"`
}

// IPAMOptions represents ipam options.
type IPAMOptions struct {
	Driver  Driver       `json:",omitempty"`
	Configs []IPAMConfig `json:",omitempty"`
}

// IPAMConfig represents ipam configuration.
type IPAMConfig struct {
	Family   IPAMConfigFamily  `json:",omitempty"`
	Subnet   string            `json:",omitempty"`
	Range    string            `json:",omitempty"`
	Gateway  string            `json:",omitempty"`
	Reserved map[string]string `json:",omitempty"`
}

const (
	// IPAMConfigFamilyUnknown UNKNOWN
	IPAMConfigFamilyUnknown = "UNKNOWN"
	// IPAMConfigFamilyIpv4 IPV4
	IPAMConfigFamilyIpv4 = "IPV4"
	// IPAMConfigFamilyIpv6 IPV6
	IPAMConfigFamilyIpv6 = "IPV6"
)

// IPAMConfigFamily represents the family of a ipam configuration.
type IPAMConfigFamily string

// Driver represents a driver (network/volume).
type Driver struct {
	Name    string            `json:",omitempty"`
	Options map[string]string `json:",omitempty"`
}
