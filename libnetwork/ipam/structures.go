package ipam

import (
	"fmt"
	"net"
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
	Pool     *net.IPNet
	addrs    *bitmap.Bitmap
	children map[string]struct{}

	// Whether to implicitly release the pool once it no longer has any children.
	autoRelease bool
}

// SubnetKey is the composite key to an address pool within an address space.
type SubnetKey struct {
	Subnet, ChildSubnet string
}

// addrSpace contains the pool configurations for the address space
type addrSpace struct {
	// Master subnet pools, indexed by the value's stringified PoolData.Pool field.
	subnets map[string]*PoolData

	// Predefined pool for the address space
	predefined           []*net.IPNet
	predefinedStartIndex int

	sync.Mutex
}

// AddressRange specifies first and last ip ordinal which
// identifies a range in a pool of addresses
type AddressRange struct {
	Sub        *net.IPNet
	Start, End uint64
}

// String returns the string form of the AddressRange object
func (r *AddressRange) String() string {
	return fmt.Sprintf("Sub: %s, range [%d, %d]", r.Sub, r.Start, r.End)
}

// String returns the string form of the SubnetKey object
func (s *PoolID) String() string {
	k := fmt.Sprintf("%s/%s", s.AddressSpace, s.Subnet)
	if s.ChildSubnet != "" {
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
	s.AddressSpace = p[0]
	s.Subnet = fmt.Sprintf("%s/%s", p[1], p[2])
	if len(p) == 5 {
		s.ChildSubnet = fmt.Sprintf("%s/%s", p[3], p[4])
	}

	return nil
}

// String returns the string form of the PoolData object
func (p *PoolData) String() string {
	return fmt.Sprintf("Pool: %s, Children: %d",
		p.Pool.String(), len(p.children))
}

// allocateSubnet adds the subnet k to the address space.
func (aSpace *addrSpace) allocateSubnet(nw, sub *net.IPNet) (SubnetKey, error) {
	aSpace.Lock()
	defer aSpace.Unlock()

	// Check if already allocated
	if pool, ok := aSpace.subnets[nw.String()]; ok {
		var childExists bool
		if sub != nil {
			_, childExists = pool.children[sub.String()]
		}
		if sub == nil || childExists {
			// This means the same pool is already allocated. allocateSubnet is called when there
			// is request for a pool/subpool. It should ensure there is no overlap with existing pools
			return SubnetKey{}, ipamapi.ErrPoolOverlap
		}
	}

	return aSpace.allocateSubnetL(nw, sub)
}

func (aSpace *addrSpace) allocateSubnetL(nw, sub *net.IPNet) (SubnetKey, error) {
	// If master pool, check for overlap
	if sub == nil {
		if aSpace.contains(nw) {
			return SubnetKey{}, ipamapi.ErrPoolOverlap
		}
		k := SubnetKey{Subnet: nw.String()}
		// This is a new master pool, add it along with corresponding bitmask
		aSpace.subnets[k.Subnet] = newPoolData(nw)
		return k, nil
	}

	// This is a new non-master pool (subPool)

	_, err := getAddressRange(sub, nw)
	if err != nil {
		return SubnetKey{}, err
	}

	k := SubnetKey{Subnet: nw.String(), ChildSubnet: sub.String()}

	// Look for parent pool
	pp, ok := aSpace.subnets[k.Subnet]
	if !ok {
		// Parent pool does not exist, add it along with corresponding bitmask
		pp = newPoolData(nw)
		pp.autoRelease = true
		aSpace.subnets[k.Subnet] = pp
	}
	pp.children[k.ChildSubnet] = struct{}{}
	return k, nil
}

func (aSpace *addrSpace) releaseSubnet(k SubnetKey) error {
	aSpace.Lock()
	defer aSpace.Unlock()

	p, ok := aSpace.subnets[k.Subnet]
	if !ok {
		return ipamapi.ErrBadPool
	}

	if k.ChildSubnet != "" {
		if _, ok := p.children[k.ChildSubnet]; !ok {
			return ipamapi.ErrBadPool
		}
		delete(p.children, k.ChildSubnet)
	} else {
		p.autoRelease = true
	}

	if len(p.children) == 0 && p.autoRelease {
		delete(aSpace.subnets, k.Subnet)
	}

	return nil
}

// contains checks whether nw is a superset or subset of any of the existing subnets in this address space.
func (aSpace *addrSpace) contains(nw *net.IPNet) bool {
	for _, v := range aSpace.subnets {
		if nw.Contains(v.Pool.IP) || v.Pool.Contains(nw.IP) {
			return true
		}
	}
	return false
}
