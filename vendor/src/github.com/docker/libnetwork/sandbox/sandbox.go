package sandbox

import (
	"net"

	"github.com/docker/libnetwork/netutils"
)

// Sandbox represents a network sandbox, identified by a specific key.  It
// holds a list of Interfaces, routes etc, and more can be added dynamically.
type Sandbox interface {
	// The path where the network namespace is mounted.
	Key() string

	// The collection of Interface previously added with the AddInterface
	// method. Note that this doesn't incude network interfaces added in any
	// other way (such as the default loopback interface which are automatically
	// created on creation of a sandbox).
	Interfaces() []*Interface

	// Add an existing Interface to this sandbox. The operation will rename
	// from the Interface SrcName to DstName as it moves, and reconfigure the
	// interface according to the specified settings.
	AddInterface(*Interface) error

	// Remove an interface from the sandbox by renamin to original name
	// and moving it out of the sandbox.
	RemoveInterface(*Interface) error

	// Set default IPv4 gateway for the sandbox
	SetGateway(gw net.IP) error

	// Set default IPv6 gateway for the sandbox
	SetGatewayIPv6(gw net.IP) error

	// Destroy the sandbox
	Destroy() error
}

// Info represents all possible information that
// the driver wants to place in the sandbox which includes
// interfaces, routes and gateway
type Info struct {
	Interfaces []*Interface

	// IPv4 gateway for the sandbox.
	Gateway net.IP

	// IPv6 gateway for the sandbox.
	GatewayIPv6 net.IP

	// TODO: Add routes and ip tables etc.
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
func (s *Info) GetCopy() *Info {
	list := make([]*Interface, len(s.Interfaces))
	for i, iface := range s.Interfaces {
		list[i] = iface.GetCopy()
	}
	gw := netutils.GetIPCopy(s.Gateway)
	gw6 := netutils.GetIPCopy(s.GatewayIPv6)

	return &Info{Interfaces: list, Gateway: gw, GatewayIPv6: gw6}
}

// Equal checks if this instance of SandboxInfo is equal to the passed one
func (s *Info) Equal(o *Info) bool {
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
