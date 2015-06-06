package driverapi

import (
	"net"

	"github.com/docker/libnetwork/types"
)

// NetworkPluginEndpointType represents the Endpoint Type used by Plugin system
const NetworkPluginEndpointType = "NetworkDriver"

// Driver is an interface that every plugin driver needs to implement.
type Driver interface {
	// Push driver specific config to the driver
	Config(options map[string]interface{}) error

	// CreateNetwork invokes the driver method to create a network passing
	// the network id and network specific config. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateNetwork(nid types.UUID, options map[string]interface{}) error

	// DeleteNetwork invokes the driver method to delete network passing
	// the network id.
	DeleteNetwork(nid types.UUID) error

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id endpoint information and driver
	// specific config. The endpoint information can be either consumed by
	// the driver or populated by the driver. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateEndpoint(nid, eid types.UUID, epInfo EndpointInfo, options map[string]interface{}) error

	// DeleteEndpoint invokes the driver method to delete an endpoint
	// passing the network id and endpoint id.
	DeleteEndpoint(nid, eid types.UUID) error

	// EndpointOperInfo retrieves from the driver the operational data related to the specified endpoint
	EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error)

	// Join method is invoked when a Sandbox is attached to an endpoint.
	Join(nid, eid types.UUID, sboxKey string, jinfo JoinInfo, options map[string]interface{}) error

	// Leave method is invoked when a Sandbox detaches from an endpoint.
	Leave(nid, eid types.UUID) error

	// Type returns the the type of this driver, the network type this driver manages
	Type() string
}

// EndpointInfo provides a go interface to fetch or populate endpoint assigned network resources.
type EndpointInfo interface {
	// Interfaces returns a list of interfaces bound to the endpoint.
	// If the list is not empty the driver is only expected to consume the interfaces.
	// It is an error to try to add interfaces to a non-empty list.
	// If the list is empty the driver is expected to populate with 0 or more interfaces.
	Interfaces() []InterfaceInfo

	// AddInterface is used by the driver to add an interface to the interface list.
	// This method will return an error if the driver attempts to add interfaces
	// if the Interfaces() method returned a non-empty list.
	// ID field need only have significance within the endpoint so it can be a simple
	// monotonically increasing number
	AddInterface(ID int, mac net.HardwareAddr, ipv4 net.IPNet, ipv6 net.IPNet) error
}

// InterfaceInfo provides a go interface for drivers to retrive
// network information to interface resources.
type InterfaceInfo interface {
	// MacAddress returns the MAC address.
	MacAddress() net.HardwareAddr

	// Address returns the IPv4 address.
	Address() net.IPNet

	// AddressIPv6 returns the IPv6 address.
	AddressIPv6() net.IPNet

	// ID returns the numerical id of the interface and has significance only within
	// the endpoint.
	ID() int
}

// InterfaceNameInfo provides a go interface for the drivers to assign names
// to interfaces.
type InterfaceNameInfo interface {
	// SetNames method assigns the srcName and dstPrefix for the interface.
	SetNames(srcName, dstPrefix string) error

	// ID returns the numerical id that was assigned to the interface by the driver
	// CreateEndpoint.
	ID() int
}

// JoinInfo represents a set of resources that the driver has the ability to provide during
// join time.
type JoinInfo interface {
	// InterfaceNames returns a list of InterfaceNameInfo go interface to facilitate
	// setting the names for the interfaces.
	InterfaceNames() []InterfaceNameInfo

	// SetGateway sets the default IPv4 gateway when a container joins the endpoint.
	SetGateway(net.IP) error

	// SetGatewayIPv6 sets the default IPv6 gateway when a container joins the endpoint.
	SetGatewayIPv6(net.IP) error

	// AddStaticRoute adds a routes to the sandbox.
	// It may be used in addtion to or instead of a default gateway (as above).
	AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP, interfaceID int) error

	// SetHostsPath sets the overriding /etc/hosts path to use for the container.
	SetHostsPath(string) error

	// SetResolvConfPath sets the overriding /etc/resolv.conf path to use for the container.
	SetResolvConfPath(string) error
}

// DriverCallback provides a Callback interface for Drivers into LibNetwork
type DriverCallback interface {
	// RegisterDriver provides a way for Remote drivers to dynamically register new NetworkType and associate with a driver instance
	RegisterDriver(name string, driver Driver, capability Capability) error
}

// Scope indicates the drivers scope capability
type Scope int

const (
	// LocalScope represents the driver capable of providing networking services for containers in a single host
	LocalScope Scope = iota
	// GlobalScope represents the driver capable of providing networking services for containers across hosts
	GlobalScope
)

// Capability represents the high level capabilities of the drivers which libnetwork can make use of
type Capability struct {
	Scope Scope
}
