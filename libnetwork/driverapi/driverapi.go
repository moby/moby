package driverapi

import (
	"errors"
	"net"
)

var (
	// ErrEndpointExists is returned if more than one endpoint is added to the network
	ErrEndpointExists = errors.New("Endpoint already exists (Only one endpoint allowed)")
	// ErrNoNetwork is returned if no network with the specified id exists
	ErrNoNetwork = errors.New("No network exists")
	// ErrNoEndpoint is returned if no endpoint with the specified id exists
	ErrNoEndpoint = errors.New("No endpoint exists")
)

// UUID represents a globally unique ID of various resources like network and endpoint
type UUID string

// Driver is an interface that every plugin driver needs to implement.
type Driver interface {
	// CreateNetwork invokes the driver method to create a network passing
	// the network id and driver specific config. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateNetwork(nid UUID, config interface{}) error

	// DeleteNetwork invokes the driver method to delete network passing
	// the network id.
	DeleteNetwork(nid UUID) error

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id, sandbox key and driver
	// specific config. The config mechanism will eventually be replaced
	// with labels which are yet to be introduced.
	CreateEndpoint(nid, eid UUID, key string, config interface{}) (*SandboxInfo, error)

	// DeleteEndpoint invokes the driver method to delete an endpoint
	// passing the network id and endpoint id.
	DeleteEndpoint(nid, eid UUID) error
}

// Interface represents the settings and identity of a network device. It is
// used as a return type for Network.Link, and it is common practice for the
// caller to use this information when moving interface SrcName from host
// namespace to DstName in a different net namespace with the appropriate
// network settings.
type Interface struct {
	// The name of the interface in the origin network namespace.
	SrcName string

	// The name that will be assigned to the interface once moves inside a
	// network namespace.
	DstName string

	// IPv4 address for the interface.
	Address net.IPNet

	// IPv6 address for the interface.
	AddressIPv6 net.IPNet
}

// SandboxInfo represents all possible information that
// the driver wants to place in the sandbox which includes
// interfaces, routes and gateway
type SandboxInfo struct {
	Interfaces []*Interface

	// IPv4 gateway for the sandbox.
	Gateway net.IP

	// IPv6 gateway for the sandbox.
	GatewayIPv6 net.IP

	// TODO: Add routes and ip tables etc.
}
