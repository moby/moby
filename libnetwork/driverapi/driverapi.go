package driverapi

import (
	"errors"
	"net"

	"github.com/docker/libnetwork/netutils"
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
	// Push driver specific config to the driver
	Config(config interface{}) error

	// CreateNetwork invokes the driver method to create a network passing
	// the network id and network specific config. The config mechanism will
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
	Address *net.IPNet

	// IPv6 address for the interface.
	AddressIPv6 *net.IPNet
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

// GetCopy returns a copy of this Interface structure
func (i *Interface) GetCopy() *Interface {
	return &Interface{
		SrcName:     i.SrcName,
		DstName:     i.DstName,
		Address:     netutils.GetIPNetCopy(i.Address),
		AddressIPv6: netutils.GetIPNetCopy(i.AddressIPv6),
	}
}

// Equal checks if this instance of Interface is equal to the passed one
func (i *Interface) Equal(o *Interface) bool {
	if i == o {
		return true
	}

	if o == nil {
		return false
	}

	if i.SrcName != o.SrcName || i.DstName != o.DstName {
		return false
	}

	if !netutils.CompareIPNet(i.Address, o.Address) {
		return false
	}

	if !netutils.CompareIPNet(i.AddressIPv6, o.AddressIPv6) {
		return false
	}

	return true
}

// GetCopy returns a copy of this SandboxInfo structure
func (s *SandboxInfo) GetCopy() *SandboxInfo {
	list := make([]*Interface, len(s.Interfaces))
	for i, iface := range s.Interfaces {
		list[i] = iface.GetCopy()
	}
	gw := netutils.GetIPCopy(s.Gateway)
	gw6 := netutils.GetIPCopy(s.GatewayIPv6)

	return &SandboxInfo{Interfaces: list, Gateway: gw, GatewayIPv6: gw6}
}

// Equal checks if this instance of SandboxInfo is equal to the passed one
func (s *SandboxInfo) Equal(o *SandboxInfo) bool {
	if s == o {
		return true
	}

	if o == nil {
		return false
	}

	if !s.Gateway.Equal(o.Gateway) {
		return false
	}

	if !s.GatewayIPv6.Equal(o.GatewayIPv6) {
		return false
	}

	if (s.Interfaces == nil && o.Interfaces != nil) ||
		(s.Interfaces != nil && o.Interfaces == nil) ||
		(len(s.Interfaces) != len(o.Interfaces)) {
		return false
	}

	// Note: At the moment, the two lists must be in the same order
	for i := 0; i < len(s.Interfaces); i++ {
		if !s.Interfaces[i].Equal(o.Interfaces[i]) {
			return false
		}
	}

	return true

}
