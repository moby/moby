package network

// Network defines configuration for a container's networking stack
//
// The network configuration can be omited from a container causing the
// container to be setup with the host's networking stack
type Network struct {
	// Type sets the networks type, commonly veth and loopback
	Type string `json:"type,omitempty"`

	// Path to network namespace
	NsPath string `json:"ns_path,omitempty"`

	// The bridge to use.
	Bridge string `json:"bridge,omitempty"`

	// Prefix for the veth interfaces.
	VethPrefix string `json:"veth_prefix,omitempty"`

	// Address contains the IP and mask to set on the network interface
	Address string `json:"address,omitempty"`

	// Gateway sets the gateway address that is used as the default for the interface
	Gateway string `json:"gateway,omitempty"`

	// Mtu sets the mtu value for the interface and will be mirrored on both the host and
	// container's interfaces if a pair is created, specifically in the case of type veth
	// Note: This does not apply to loopback interfaces.
	Mtu int `json:"mtu,omitempty"`
}

// Struct describing the network specific runtime state that will be maintained by libcontainer for all running containers
// Do not depend on it outside of libcontainer.
type NetworkState struct {
	// The name of the veth interface on the Host.
	VethHost string `json:"veth_host,omitempty"`
	// The name of the veth interface created inside the container for the child.
	VethChild string `json:"veth_child,omitempty"`
	// Net namespace path.
	NsPath string `json:"ns_path,omitempty"`
}
