// Package null implements the null ipam driver. Null ipam driver satisfies ipamapi contract,
// but does not effectively reserve/allocate any address pool or address
package null

import (
	"net"
	"net/netip"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

const (
	// DriverName is the name of the built-in null ipam driver
	DriverName = "null"

	defaultAddressSpace = "null"
	defaultPoolCIDR     = "0.0.0.0/0"
	defaultPoolID       = defaultAddressSpace + "/" + defaultPoolCIDR
)

var defaultPool = netip.MustParsePrefix(defaultPoolCIDR)

type allocator struct{}

func (a *allocator) GetDefaultAddressSpaces() (string, string, error) {
	return defaultAddressSpace, defaultAddressSpace, nil
}

func (a *allocator) RequestPool(req ipamapi.PoolRequest) (ipamapi.AllocatedPool, error) {
	if req.AddressSpace != defaultAddressSpace {
		return ipamapi.AllocatedPool{}, types.InvalidParameterErrorf("unknown address space: %s", req.AddressSpace)
	}
	if req.Pool != "" {
		return ipamapi.AllocatedPool{}, types.InvalidParameterErrorf("null ipam driver does not handle specific address pool requests")
	}
	if req.SubPool != "" {
		return ipamapi.AllocatedPool{}, types.InvalidParameterErrorf("null ipam driver does not handle specific address subpool requests")
	}
	if req.V6 {
		return ipamapi.AllocatedPool{}, types.InvalidParameterErrorf("null ipam driver does not handle IPv6 address pool requests")
	}
	return ipamapi.AllocatedPool{
		PoolID: defaultPoolID,
		Pool:   defaultPool,
	}, nil
}

func (a *allocator) ReleasePool(poolID string) error {
	return nil
}

func (a *allocator) RequestAddress(poolID string, ip net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	if poolID != defaultPoolID {
		return nil, nil, types.InvalidParameterErrorf("unknown pool id: %s", poolID)
	}
	return nil, nil, nil
}

func (a *allocator) ReleaseAddress(poolID string, ip net.IP) error {
	if poolID != defaultPoolID {
		return types.InvalidParameterErrorf("unknown pool id: %s", poolID)
	}
	return nil
}

func (a *allocator) IsBuiltIn() bool {
	return true
}

// Register registers the null ipam driver with r.
func Register(r ipamapi.Registerer) error {
	return r.RegisterIpamDriver(DriverName, &allocator{})
}
