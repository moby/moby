package defaultipam

import (
	"context"
	"fmt"
	"net/netip"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

// addrSpace contains the pool configurations for the address space
type addrSpace struct {
	// Master subnet pools, indexed by the value's stringified PoolData.Pool field.
	subnets map[netip.Prefix]*PoolData

	// Predefined pool for the address space
	predefined           []netip.Prefix
	predefinedStartIndex int

	mu sync.Mutex
}

func newAddrSpace(predefined []netip.Prefix) (*addrSpace, error) {
	return &addrSpace{
		subnets:    map[netip.Prefix]*PoolData{},
		predefined: predefined,
	}, nil
}

// allocateSubnet adds the subnet k to the address space.
func (aSpace *addrSpace) allocateSubnet(nw, sub netip.Prefix) error {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	// Check if already allocated
	if pool, ok := aSpace.subnets[nw]; ok {
		var childExists bool
		if sub != (netip.Prefix{}) {
			_, childExists = pool.children[sub]
		}
		if sub == (netip.Prefix{}) || childExists {
			// This means the same pool is already allocated. allocateSubnet is called when there
			// is request for a pool/subpool. It should ensure there is no overlap with existing pools
			return ipamapi.ErrPoolOverlap
		}
	}

	return aSpace.allocateSubnetL(nw, sub)
}

func (aSpace *addrSpace) allocateSubnetL(nw, sub netip.Prefix) error {
	// If master pool, check for overlap
	if sub == (netip.Prefix{}) {
		if aSpace.overlaps(nw) {
			return ipamapi.ErrPoolOverlap
		}
		// This is a new master pool, add it along with corresponding bitmask
		aSpace.subnets[nw] = newPoolData(nw)
		return nil
	}

	// This is a new non-master pool (subPool)
	if nw.Addr().BitLen() != sub.Addr().BitLen() {
		return fmt.Errorf("pool and subpool are of incompatible address families")
	}

	// Look for parent pool
	pp, ok := aSpace.subnets[nw]
	if !ok {
		// Parent pool does not exist, add it along with corresponding bitmask
		pp = newPoolData(nw)
		pp.autoRelease = true
		aSpace.subnets[nw] = pp
	}
	pp.children[sub] = struct{}{}
	return nil
}

// overlaps reports whether nw contains any IP addresses in common with any of
// the existing subnets in this address space.
func (aSpace *addrSpace) overlaps(nw netip.Prefix) bool {
	for pool := range aSpace.subnets {
		if pool.Overlaps(nw) {
			return true
		}
	}
	return false
}

// getPredefineds returns the predefined subnets for the address space.
//
// It should not be called concurrently with any other method on the addrSpace.
func (aSpace *addrSpace) getPredefineds() []netip.Prefix {
	i := aSpace.predefinedStartIndex
	// defensive in case the list changed since last update
	if i >= len(aSpace.predefined) {
		i = 0
	}
	return append(aSpace.predefined[i:], aSpace.predefined[:i]...)
}

// updatePredefinedStartIndex rotates the predefined subnet list by amt.
//
// It should not be called concurrently with any other method on the addrSpace.
func (aSpace *addrSpace) updatePredefinedStartIndex(amt int) {
	i := aSpace.predefinedStartIndex + amt
	if i < 0 || i >= len(aSpace.predefined) {
		i = 0
	}
	aSpace.predefinedStartIndex = i
}

func (aSpace *addrSpace) allocatePredefinedPool(ipV6 bool) (netip.Prefix, error) {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	for i, nw := range aSpace.getPredefineds() {
		if ipV6 != nw.Addr().Is6() {
			continue
		}
		// Checks whether pool has already been allocated
		if _, ok := aSpace.subnets[nw]; ok {
			continue
		}
		// Shouldn't be necessary, but check prevents IP collisions should
		// predefined pools overlap for any reason.
		if !aSpace.overlaps(nw) {
			aSpace.updatePredefinedStartIndex(i + 1)
			err := aSpace.allocateSubnetL(nw, netip.Prefix{})
			if err != nil {
				return netip.Prefix{}, err
			}
			return nw, nil
		}
	}

	v := 4
	if ipV6 {
		v = 6
	}
	return netip.Prefix{}, types.NotFoundErrorf("could not find an available, non-overlapping IPv%d address pool among the defaults to assign to the network", v)
}

func (aSpace *addrSpace) releaseSubnet(nw, sub netip.Prefix) error {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	p, ok := aSpace.subnets[nw]
	if !ok {
		return ipamapi.ErrBadPool
	}

	if sub != (netip.Prefix{}) {
		if _, ok := p.children[sub]; !ok {
			return ipamapi.ErrBadPool
		}
		delete(p.children, sub)
	} else {
		p.autoRelease = true
	}

	if len(p.children) == 0 && p.autoRelease {
		delete(aSpace.subnets, nw)
	}

	return nil
}

func (aSpace *addrSpace) requestAddress(nw, sub netip.Prefix, prefAddress netip.Addr, opts map[string]string) (netip.Addr, error) {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	p, ok := aSpace.subnets[nw]
	if !ok {
		return netip.Addr{}, types.NotFoundErrorf("cannot find address pool for poolID:%v/%v", nw, sub)
	}

	if prefAddress != (netip.Addr{}) && !nw.Contains(prefAddress) {
		return netip.Addr{}, ipamapi.ErrIPOutOfRange
	}

	if sub != (netip.Prefix{}) {
		if _, ok := p.children[sub]; !ok {
			return netip.Addr{}, types.NotFoundErrorf("cannot find address pool for poolID:%v/%v", nw, sub)
		}
	}

	// In order to request for a serial ip address allocation, callers can pass in the option to request
	// IP allocation serially or first available IP in the subnet
	serial := opts[ipamapi.AllocSerialPrefix] == "true"
	ip, err := getAddress(nw, p.addrs, prefAddress, sub, serial)
	if err != nil {
		return netip.Addr{}, err
	}

	return ip, nil
}

func (aSpace *addrSpace) releaseAddress(nw, sub netip.Prefix, address netip.Addr) error {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	p, ok := aSpace.subnets[nw]
	if !ok {
		return types.NotFoundErrorf("cannot find address pool for %v/%v", nw, sub)
	}
	if sub != (netip.Prefix{}) {
		if _, ok := p.children[sub]; !ok {
			return types.NotFoundErrorf("cannot find address pool for poolID:%v/%v", nw, sub)
		}
	}

	if !address.IsValid() {
		return types.InvalidParameterErrorf("invalid address")
	}

	if !nw.Contains(address) {
		return ipamapi.ErrIPOutOfRange
	}

	defer log.G(context.TODO()).Debugf("Released address Address:%v Sequence:%s", address, p.addrs)

	return p.addrs.Unset(netiputil.HostID(address, uint(nw.Bits())))
}
