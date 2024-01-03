// Package null implements the null ipam driver. Null ipam driver satisfies ipamapi contract,
// but does not effectively reserve/allocate any address pool or address
package null

import (
	"net"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

const (
	defaultAddressSpace = "null"
	defaultPoolCIDR     = "0.0.0.0/0"
	defaultPoolID       = defaultAddressSpace + "/" + defaultPoolCIDR
)

var defaultPool, _ = types.ParseCIDR(defaultPoolCIDR)

type allocator struct{}

func (a *allocator) GetDefaultAddressSpaces() (string, string, error) {
	return defaultAddressSpace, defaultAddressSpace, nil
}

func (a *allocator) RequestPool(addressSpace, requestedPool, requestedSubPool string, _ map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	if addressSpace != defaultAddressSpace {
		return "", nil, nil, types.InvalidParameterErrorf("unknown address space: %s", addressSpace)
	}
	if requestedPool != "" {
		return "", nil, nil, types.InvalidParameterErrorf("null ipam driver does not handle specific address pool requests")
	}
	if requestedSubPool != "" {
		return "", nil, nil, types.InvalidParameterErrorf("null ipam driver does not handle specific address subpool requests")
	}
	if v6 {
		return "", nil, nil, types.InvalidParameterErrorf("null ipam driver does not handle IPv6 address pool pool requests")
	}
	return defaultPoolID, defaultPool, nil, nil
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
	return r.RegisterIpamDriver(ipamapi.NullIPAM, &allocator{})
}
