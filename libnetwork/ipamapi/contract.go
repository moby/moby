// Package ipamapi specifies the contract the IPAM service (built-in or remote) needs to satisfy.
package ipamapi

import (
	"net"
	"net/netip"

	"github.com/docker/docker/libnetwork/types"
)

// IPAM plugin types
const (
	// PluginEndpointType represents the Endpoint Type used by Plugin system
	PluginEndpointType = "IpamDriver"
	// RequestAddressType represents the Address Type used when requesting an address
	RequestAddressType = "RequestAddressType"
)

// Registerer provides a callback interface for registering IPAM instances into libnetwork.
type Registerer interface {
	// RegisterIpamDriver provides a way for drivers to dynamically register with libnetwork
	RegisterIpamDriver(name string, driver Ipam) error
	// RegisterIpamDriverWithCapabilities provides a way for drivers to dynamically register with libnetwork and specify capabilities
	RegisterIpamDriverWithCapabilities(name string, driver Ipam, capability *Capability) error
}

// Well-known errors returned by IPAM
var (
	ErrInvalidAddressSpace = types.InvalidParameterErrorf("invalid address space")
	ErrInvalidPool         = types.InvalidParameterErrorf("invalid address pool")
	ErrInvalidSubPool      = types.InvalidParameterErrorf("invalid address subpool")
	ErrNoAvailableIPs      = types.UnavailableErrorf("no available addresses on this pool")
	ErrNoIPReturned        = types.UnavailableErrorf("no address returned")
	ErrIPAlreadyAllocated  = types.ForbiddenErrorf("Address already in use")
	ErrIPOutOfRange        = types.InvalidParameterErrorf("requested address is out of range")
	ErrPoolOverlap         = types.ForbiddenErrorf("Pool overlaps with other one on this address space")
	ErrBadPool             = types.InvalidParameterErrorf("address space does not contain specified address pool")
	ErrNoMoreSubnets       = types.InvalidParameterErrorf("all predefined address pools have been fully subnetted")
)

// Ipam represents the interface the IPAM service plugins must implement
// in order to allow injection/modification of IPAM database.
type Ipam interface {
	// GetDefaultAddressSpaces returns the default local and global address spaces for this ipam
	GetDefaultAddressSpaces() (string, string, error)
	// RequestPool allocate an address pool either statically or dynamically
	// based on req.
	RequestPool(req PoolRequest) (AllocatedPool, error)
	// ReleasePool releases the address pool identified by the passed id
	ReleasePool(poolID string) error
	// RequestAddress request an address from the specified pool ID. Input options or required IP can be passed.
	RequestAddress(string, net.IP, map[string]string) (*net.IPNet, map[string]string, error)
	// ReleaseAddress releases the address from the specified pool ID.
	ReleaseAddress(string, net.IP) error

	// IsBuiltIn returns true if it is a built-in driver.
	IsBuiltIn() bool
}

type PoolRequest struct {
	// AddressSpace is a mandatory field which denotes which block of pools
	// should be used to make the allocation. This value is opaque, and only
	// the IPAM driver can interpret it. Each driver might support a different
	// set of AddressSpace.
	AddressSpace string
	// Pool is a prefix in CIDR notation. It's non-mandatory. When specified
	// the Pool will be statically allocated. The Pool is used for dynamic
	// address allocation -- except when SubPool is specified.
	Pool string
	// SubPool is a subnet from Pool, in CIDR notation too. It's non-mandatory.
	// When specified, it represents the subnet where addresses will be
	// dynamically allocated. It can't be specified if Pool isn't specified.
	SubPool string
	// Options is a map of opaque k/v passed to the driver. It's non-mandatory.
	// Drivers are free to ignore it.
	Options map[string]string
	// Exclude is a list of prefixes the requester wish to not be dynamically
	// allocated (ie. when Pool isn't specified). It's up to the IPAM driver to
	// take it into account, or totally ignore it. It's required to be sorted.
	Exclude []netip.Prefix
	// V6 indicates which address family should be used to dynamically allocate
	// a prefix (ie. when Pool isn't specified).
	V6 bool
}

type AllocatedPool struct {
	// PoolID represents the ID of the allocated pool. Its value is opaque and
	// shouldn't be interpreted by anything but the IPAM driver that generated
	// it.
	PoolID string
	// Pool is the allocated prefix.
	Pool netip.Prefix
	// Meta represents a list of extra IP addresses automatically reserved
	// during the pool allocation. These are generally keyed by well-known
	// strings defined in the netlabel package.
	Meta map[string]string
}

// Capability represents the requirements and capabilities of the IPAM driver
type Capability struct {
	// Whether on address request, libnetwork must
	// specify the endpoint MAC address
	RequiresMACAddress bool
	// Whether of daemon start, libnetwork must replay the pool
	// request and the address request for current local networks
	RequiresRequestReplay bool
}
