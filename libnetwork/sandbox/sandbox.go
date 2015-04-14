package sandbox

import (
	"net"

	"github.com/docker/libnetwork/driverapi"
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
	Interfaces() []*driverapi.Interface

	// Add an existing Interface to this sandbox. The operation will rename
	// from the Interface SrcName to DstName as it moves, and reconfigure the
	// interface according to the specified settings.
	AddInterface(*driverapi.Interface) error

	// Set default IPv4 gateway for the sandbox
	SetGateway(gw net.IP) error

	// Set default IPv6 gateway for the sandbox
	SetGatewayIPv6(gw net.IP) error
}
