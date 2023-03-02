package ipam

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

// PoolID is the pointer to the configured pools in each address space
type PoolID struct {
	AddressSpace string
	SubnetKey
}

// PoolData contains the configured pool data
type PoolData struct {
	addrs    *bitmap.Bitmap
	children map[netip.Prefix]struct{}

	// Whether to implicitly release the pool once it no longer has any children.
	autoRelease bool
}

// SubnetKey is the composite key to an address pool within an address space.
type SubnetKey struct {
	Subnet, ChildSubnet netip.Prefix
}

// addrSpace contains the pool configurations for the address space
type addrSpace struct {
	// Master subnet pools, indexed by the value's stringified PoolData.Pool field.
	subnets map[netip.Prefix]*PoolData

	// Predefined pool for the address space
	predefined           []netip.Prefix
	predefinedStartIndex int

	sync.Mutex
}

// String returns the string form of the SubnetKey object
func (s *PoolID) String() string {
	k := fmt.Sprintf("%s/%s", s.AddressSpace, s.Subnet)
	if s.ChildSubnet != (netip.Prefix{}) {
		k = fmt.Sprintf("%s/%s", k, s.ChildSubnet)
	}
	return k
}

// FromString populates the SubnetKey object reading it from string
func (s *PoolID) FromString(str string) error {
	if str == "" || !strings.Contains(str, "/") {
		return types.BadRequestErrorf("invalid string form for subnetkey: %s", str)
	}

	p := strings.Split(str, "/")
	if len(p) != 3 && len(p) != 5 {
		return types.BadRequestErrorf("invalid string form for subnetkey: %s", str)
	}
	sub, err := netip.ParsePrefix(p[1] + "/" + p[2])
	if err != nil {
		return types.BadRequestErrorf("%v", err)
	}
	var child netip.Prefix
	if len(p) == 5 {
		child, err = netip.ParsePrefix(p[3] + "/" + p[4])
		if err != nil {
			return types.BadRequestErrorf("%v", err)
		}
	}

	*s = PoolID{
		AddressSpace: p[0],
		SubnetKey: SubnetKey{
			Subnet:      sub,
			ChildSubnet: child,
		},
	}
	return nil
}

// String returns the string form of the PoolData object
func (p *PoolData) String() string {
	return fmt.Sprintf("PoolData[Children: %d]", len(p.children))
}

// allocateSubnet adds the subnet k to the address space.
func (aSpace *addrSpace) allocateSubnet(nw, sub netip.Prefix) error {
	aSpace.Lock()
	defer aSpace.Unlock()

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
		if aSpace.contains(nw) {
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

func (aSpace *addrSpace) releaseSubnet(nw, sub netip.Prefix) error {
	aSpace.Lock()
	defer aSpace.Unlock()

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

// contains checks whether nw is a superset or subset of any of the existing subnets in this address space.
func (aSpace *addrSpace) contains(nw netip.Prefix) bool {
	for pool := range aSpace.subnets {
		if nw.Contains(pool.Addr()) || pool.Contains(nw.Addr()) {
			return true
		}
	}
	return false
}
