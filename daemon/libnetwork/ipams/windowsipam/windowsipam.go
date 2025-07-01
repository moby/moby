//go:build windows

package windowsipam

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

const (
	localAddressSpace  = "LocalDefault"
	globalAddressSpace = "GlobalDefault"
)

// DefaultIPAM defines the default ipam-driver for local-scoped windows networks
const DefaultIPAM = "windows"

var defaultPool = netip.MustParsePrefix("0.0.0.0/0")

type allocator struct{}

// Register registers the built-in ipam service with libnetwork
func Register(r ipamapi.Registerer) error {
	return r.RegisterIpamDriver(DefaultIPAM, &allocator{})
}

func (a *allocator) GetDefaultAddressSpaces() (string, string, error) {
	return localAddressSpace, globalAddressSpace, nil
}

// RequestPool returns an address pool along with its unique id. This is a null ipam driver. It allocates the
// subnet user asked and does not validate anything. Doesn't support subpool allocation
func (a *allocator) RequestPool(req ipamapi.PoolRequest) (ipamapi.AllocatedPool, error) {
	log.G(context.TODO()).Debugf("RequestPool: %+v", req)
	if req.SubPool != "" || req.V6 {
		return ipamapi.AllocatedPool{}, types.InternalErrorf("this request is not supported by the 'windows' ipam driver")
	}

	pool := defaultPool
	if req.Pool != "" {
		var err error
		if pool, err = netip.ParsePrefix(req.Pool); err != nil {
			return ipamapi.AllocatedPool{}, fmt.Errorf("invalid IPAM request: %w", err)
		}
	}

	return ipamapi.AllocatedPool{
		PoolID: pool.String(),
		Pool:   pool,
	}, nil
}

// ReleasePool releases the address pool - always succeeds
func (a *allocator) ReleasePool(poolID string) error {
	log.G(context.TODO()).Debugf("ReleasePool(%s)", poolID)
	return nil
}

// RequestAddress returns an address from the specified pool ID.
// Always allocate the 0.0.0.0/32 ip if no preferred address was specified
func (a *allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	log.G(context.TODO()).Debugf("RequestAddress(%s, %v, %v)", poolID, prefAddress, opts)
	_, ipNet, err := net.ParseCIDR(poolID)
	if err != nil {
		return nil, nil, err
	}

	if prefAddress != nil {
		return &net.IPNet{IP: prefAddress, Mask: ipNet.Mask}, nil, nil
	}

	return nil, nil, nil
}

// ReleaseAddress releases the address - always succeeds
func (a *allocator) ReleaseAddress(poolID string, address net.IP) error {
	log.G(context.TODO()).Debugf("ReleaseAddress(%s, %v)", poolID, address)
	return nil
}

func (a *allocator) IsBuiltIn() bool {
	return true
}
