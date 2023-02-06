package ipam

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

// SubnetKey is the pointer to the configured pools in each address space
type SubnetKey struct {
	AddressSpace string
	Subnet       string
	ChildSubnet  string
}

// PoolData contains the configured pool data
type PoolData struct {
	ParentKey SubnetKey
	Pool      *net.IPNet
	Range     *AddressRange `json:",omitempty"`
	RefCount  int
}

// addrSpace contains the pool configurations for the address space
type addrSpace struct {
	subnets map[SubnetKey]*PoolData
	alloc   *Allocator

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
func (s *SubnetKey) String() string {
	k := fmt.Sprintf("%s/%s", s.AddressSpace, s.Subnet)
	if s.ChildSubnet != "" {
		k = fmt.Sprintf("%s/%s", k, s.ChildSubnet)
	}
	return k
}

// FromString populates the SubnetKey object reading it from string
func (s *SubnetKey) FromString(str string) error {
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
	return fmt.Sprintf("ParentKey: %s, Pool: %s, Range: %s, RefCount: %d",
		p.ParentKey.String(), p.Pool.String(), p.Range, p.RefCount)
}

// allocateSubnet adds the subnet k to the address space.
func (aSpace *addrSpace) allocateSubnet(k SubnetKey, nw *net.IPNet, ipr *AddressRange, pdf bool) error {
	aSpace.Lock()
	defer aSpace.Unlock()

	// Check if already allocated
	if _, ok := aSpace.subnets[k]; ok {
		if pdf {
			return types.InternalMaskableErrorf("predefined pool %s is already reserved", nw)
		}
		// This means the same pool is already allocated. allocateSubnet is called when there
		// is request for a pool/subpool. It should ensure there is no overlap with existing pools
		return ipamapi.ErrPoolOverlap
	}

	// If master pool, check for overlap
	if ipr == nil {
		if aSpace.contains(k.AddressSpace, nw) {
			return ipamapi.ErrPoolOverlap
		}
		// This is a new master pool, add it along with corresponding bitmask
		aSpace.subnets[k] = &PoolData{Pool: nw, RefCount: 1}
		return aSpace.alloc.insertBitMask(k, nw)
	}

	// This is a new non-master pool (subPool)
	p := &PoolData{
		ParentKey: SubnetKey{AddressSpace: k.AddressSpace, Subnet: k.Subnet},
		Pool:      nw,
		Range:     ipr,
		RefCount:  1,
	}
	aSpace.subnets[k] = p

	// Look for parent pool
	pp, ok := aSpace.subnets[p.ParentKey]
	if ok {
		aSpace.incRefCount(pp, 1)
		return nil
	}

	// Parent pool does not exist, add it along with corresponding bitmask
	aSpace.subnets[p.ParentKey] = &PoolData{Pool: nw, RefCount: 1}
	return aSpace.alloc.insertBitMask(p.ParentKey, nw)
}

func (aSpace *addrSpace) releaseSubnet(k SubnetKey) error {
	aSpace.Lock()
	defer aSpace.Unlock()

	p, ok := aSpace.subnets[k]
	if !ok {
		return ipamapi.ErrBadPool
	}

	aSpace.incRefCount(p, -1)

	c := p
	for ok {
		if c.RefCount == 0 {
			delete(aSpace.subnets, k)
			if c.Range == nil {
				return nil
			}
		}
		k = c.ParentKey
		c, ok = aSpace.subnets[k]
	}

	return nil
}

func (aSpace *addrSpace) incRefCount(p *PoolData, delta int) {
	c := p
	ok := true
	for ok {
		c.RefCount += delta
		c, ok = aSpace.subnets[c.ParentKey]
	}
}

// Checks whether the passed subnet is a superset or subset of any of the subset in this config db
func (aSpace *addrSpace) contains(space string, nw *net.IPNet) bool {
	for k, v := range aSpace.subnets {
		if space == k.AddressSpace && k.ChildSubnet == "" {
			if nw.Contains(v.Pool.IP) || v.Pool.Contains(nw.IP) {
				return true
			}
		}
	}
	return false
}
