package driverapi

import (
	"context"
	"net"

	"github.com/moby/moby/v2/daemon/libnetwork/options"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

// NetworkPluginEndpointType represents the Endpoint Type used by Plugin system
const NetworkPluginEndpointType = "NetworkDriver"

// Driver is an interface that every plugin driver needs to implement.
type Driver interface {
	// CreateNetwork invokes the driver method to create a network
	// passing the network id and network specific config. The
	// config mechanism will eventually be replaced with labels
	// which are yet to be introduced. The driver can return a
	// list of table names for which it is interested in receiving
	// notification when a CRUD operation is performed on any
	// entry in that table. This will be ignored for local scope
	// drivers.
	CreateNetwork(ctx context.Context, nid string, options map[string]any, nInfo NetworkInfo, ipV4Data, ipV6Data []IPAMData) error

	// DeleteNetwork invokes the driver method to delete network passing
	// the network id.
	DeleteNetwork(nid string) error

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id endpoint information and driver
	// specific config. The endpoint information can be either consumed by
	// the driver or populated by the driver. The config mechanism will
	// eventually be replaced with labels which are yet to be introduced.
	CreateEndpoint(ctx context.Context, nid, eid string, ifInfo InterfaceInfo, options map[string]any) error

	// DeleteEndpoint invokes the driver method to delete an endpoint
	// passing the network id and endpoint id.
	DeleteEndpoint(nid, eid string) error

	// EndpointOperInfo retrieves from the driver the operational data related to the specified endpoint
	EndpointOperInfo(nid, eid string) (map[string]any, error)

	// Join method is invoked when a Sandbox is attached to an endpoint.
	Join(ctx context.Context, nid, eid string, sboxKey string, jinfo JoinInfo, epOpts, sbOpts map[string]any) error

	// Leave method is invoked when a Sandbox detaches from an endpoint.
	Leave(nid, eid string) error

	// Type returns the type of this driver, the network type this driver manages
	Type() string

	// IsBuiltIn returns true if it is a built-in driver
	IsBuiltIn() bool
}

// NetworkAllocator is a special kind of network driver used by cnmallocator to
// allocate resources inside a Swarm cluster.
type NetworkAllocator interface {
	// NetworkAllocate invokes the driver method to allocate network
	// specific resources passing network id and network specific config.
	// It returns a key,value pair of network specific driver allocations
	// to the caller.
	NetworkAllocate(nid string, options map[string]string, ipV4Data, ipV6Data []IPAMData) (map[string]string, error)

	// NetworkFree invokes the driver method to free network specific resources
	// associated with a given network id.
	NetworkFree(nid string) error

	// IsBuiltIn returns true if it is a built-in driver
	IsBuiltIn() bool
}

// TableWatcher is an optional interface for a network driver.
type TableWatcher interface {
	// EventNotify notifies the driver when a CRUD operation has
	// happened on a table of its interest as soon as this node
	// receives such an event in the gossip layer. This method is
	// only invoked for the global scope driver.
	EventNotify(nid string, tableName string, key string, prev, value []byte)

	// DecodeTableEntry passes the driver a key, value pair from table it registered
	// with libnetwork. Driver should return {object ID, map[string]string} tuple.
	// If DecodeTableEntry is called for a table associated with NetworkObject or
	// EndpointObject the return object ID should be the network id or endpoint id
	// associated with that entry. map should have information about the object that
	// can be presented to the user.
	// For example: overlay driver returns the VTEP IP of the host that has the endpoint
	// which is shown in 'network inspect --verbose'
	DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string)
}

// ExtConner is an optional interface for a network driver.
type ExtConner interface {
	// ProgramExternalConnectivity tells the driver the ids of the endpoints
	// currently acting as the container's default gateway for IPv4 and IPv6,
	// passed as gw4Id/gw6Id. (Those endpoints may be managed by different network
	// drivers. If there is no gateway, the id will be the empty string.)
	//
	// This method is called after Driver.Join, before Driver.Leave, and when eid
	// is or was equal to gw4Id or gw6Id, and there's a change. It may also be
	// called when the gateways have not changed.
	//
	// When an endpoint acting as a gateway is deleted, this function is called
	// with that endpoint's id in eid, and empty gateway ids (even if another
	// is present and will shortly be selected as the gateway).
	ProgramExternalConnectivity(ctx context.Context, nid, eid string, gw4Id, gw6Id string) error
}

// IPv6Releaser is an optional interface for a network driver.
type IPv6Releaser interface {
	// ReleaseIPv6 tells the driver that an endpoint has no IPv6 address, even
	// if the options passed to Driver.CreateEndpoint specified an address. This
	// happens when, for example, sysctls applied after configuring the interface
	// disable IPv6.
	ReleaseIPv6(ctx context.Context, nid, eid string) error
}

// GwAllocChecker is an optional interface for a network driver.
type GwAllocChecker interface {
	// GetSkipGwAlloc returns true if the opts describe a network
	// that does not need a gateway IPv4/IPv6 address, else false.
	GetSkipGwAlloc(opts options.Generic) (skipIPv4, skipIPv6 bool, err error)
}

// NetworkInfo provides a go interface for drivers to provide network
// specific information to libnetwork.
type NetworkInfo interface {
	// TableEventRegister registers driver interest in a given
	// table name.
	TableEventRegister(tableName string, objType ObjectType) error
}

// InterfaceInfo provides a go interface for drivers to retrieve
// network information to interface resources.
type InterfaceInfo interface {
	// SetMacAddress allows the driver to set the mac address to the endpoint interface
	// during the call to CreateEndpoint, if the mac address is not already set.
	SetMacAddress(mac net.HardwareAddr) error

	// SetIPAddress allows the driver to set the ip address to the endpoint interface
	// during the call to CreateEndpoint, if the address is not already set.
	// The API is to be used to assign both the IPv4 and IPv6 address types.
	SetIPAddress(ip *net.IPNet) error

	// MacAddress returns the MAC address.
	MacAddress() net.HardwareAddr

	// Address returns the IPv4 address.
	Address() *net.IPNet

	// AddressIPv6 returns the IPv6 address.
	AddressIPv6() *net.IPNet

	// NetnsPath returns the path of the network namespace, if there is one. Else "".
	NetnsPath() string

	// SetCreatedInContainer can be called by the driver to indicate that it's
	// created the network interface in the container's network namespace (so,
	// it doesn't need to be moved there).
	SetCreatedInContainer(bool)
}

// InterfaceNameInfo provides a go interface for the drivers to assign names
// to interfaces.
type InterfaceNameInfo interface {
	// SetNames method assigns the srcName, dstPrefix, and dstName for the
	// interface. If both dstName and dstPrefix are set, dstName takes
	// precedence.
	SetNames(srcName, dstPrefix, dstName string) error
}

// JoinInfo represents a set of resources that the driver has the ability to provide during
// join time.
type JoinInfo interface {
	// InterfaceName returns an InterfaceNameInfo go interface to facilitate
	// setting the names for the interface.
	InterfaceName() InterfaceNameInfo

	// SetGateway sets the default IPv4 gateway when a container joins the endpoint.
	SetGateway(net.IP) error

	// SetGatewayIPv6 sets the default IPv6 gateway when a container joins the endpoint.
	SetGatewayIPv6(net.IP) error

	// AddStaticRoute adds a route to the sandbox.
	// It may be used in addition to or instead of a default gateway (as above).
	AddStaticRoute(destination *net.IPNet, routeType types.RouteType, nextHop net.IP) error

	// DisableGatewayService tells libnetwork not to provide Default GW for the container
	DisableGatewayService()

	// ForceGw4 may be called by a driver to indicate that, even if it has not set up
	// an IPv4 gateway, libnet should assume the endpoint has external IPv4 connectivity.
	ForceGw4()

	// ForceGw6 may be called by a driver to indicate that, even if it has not set up
	// an IPv6 gateway, libnet should assume the endpoint has external IPv6 connectivity.
	ForceGw6()

	// AddTableEntry adds a table entry to the gossip layer
	// passing the table name, key and an opaque value.
	AddTableEntry(tableName string, key string, value []byte) error
}

// Registerer provides a way for network drivers to be dynamically registered.
type Registerer interface {
	RegisterDriver(name string, driver Driver, capability Capability) error
	RegisterNetworkAllocator(name string, driver NetworkAllocator) error
}

// Capability represents the high level capabilities of the drivers which libnetwork can make use of
type Capability struct {
	DataScope         string
	ConnectivityScope string
}

// IPAMData represents the per-network ip related
// operational information libnetwork will send
// to the network driver during CreateNetwork()
type IPAMData struct {
	AddressSpace string
	Pool         *net.IPNet
	Gateway      *net.IPNet
	AuxAddresses map[string]*net.IPNet
}

// ObjectType represents the type of object driver wants to store in libnetwork's networkDB
type ObjectType int

const (
	// EndpointObject should be set for libnetwork endpoint object related data
	EndpointObject ObjectType = 1 + iota
	// NetworkObject should be set for libnetwork network object related data
	NetworkObject
	// OpaqueObject is for driver specific data with no corresponding libnetwork object
	OpaqueObject
)

// IsValidType validates the passed in type against the valid object types
func IsValidType(objType ObjectType) bool {
	switch objType {
	case EndpointObject:
		fallthrough
	case NetworkObject:
		fallthrough
	case OpaqueObject:
		return true
	}
	return false
}
