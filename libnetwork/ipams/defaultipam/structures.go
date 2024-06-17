package defaultipam

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/docker/docker/libnetwork/bitmap"
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

func (k SubnetKey) Is6() bool {
	return k.Subnet.Addr().Is6()
}

// PoolIDFromString creates a new PoolID and populates the SubnetKey object
// reading it from the given string.
func PoolIDFromString(str string) (pID PoolID, err error) {
	if str == "" {
		return pID, types.InvalidParameterErrorf("invalid string form for subnetkey: %s", str)
	}

	p := strings.Split(str, "/")
	if len(p) != 3 && len(p) != 5 {
		return pID, types.InvalidParameterErrorf("invalid string form for subnetkey: %s", str)
	}
	pID.AddressSpace = p[0]
	pID.Subnet, err = netip.ParsePrefix(p[1] + "/" + p[2])
	if err != nil {
		return pID, types.InvalidParameterErrorf("invalid string form for subnetkey: %s", str)
	}
	if len(p) == 5 {
		pID.ChildSubnet, err = netip.ParsePrefix(p[3] + "/" + p[4])
		if err != nil {
			return pID, types.InvalidParameterErrorf("invalid string form for subnetkey: %s", str)
		}
	}

	return pID, nil
}

// String returns the string form of the SubnetKey object
func (s *PoolID) String() string {
	if s.ChildSubnet == (netip.Prefix{}) {
		return s.AddressSpace + "/" + s.Subnet.String()
	} else {
		return s.AddressSpace + "/" + s.Subnet.String() + "/" + s.ChildSubnet.String()
	}
}

// String returns the string form of the PoolData object
func (p *PoolData) String() string {
	return fmt.Sprintf("PoolData[Children: %d]", len(p.children))
}

// mergeIter is used to iterate on both 'a' and 'b' at the same time while
// maintaining the total order that would arise if both were merged and then
// sorted. Both 'a' and 'b' have to be sorted beforehand.
type mergeIter struct {
	a, b   []netip.Prefix
	ia, ib int
	cmp    func(a, b netip.Prefix) int
	lastA  bool
}

func newMergeIter(a, b []netip.Prefix, cmp func(a, b netip.Prefix) int) *mergeIter {
	iter := &mergeIter{
		a:   a,
		b:   b,
		cmp: cmp,
	}
	iter.lastA = iter.nextA()

	return iter
}

func (it *mergeIter) Get() netip.Prefix {
	if it.ia+it.ib >= len(it.a)+len(it.b) {
		return netip.Prefix{}
	}

	if it.lastA {
		return it.a[it.ia]
	}

	return it.b[it.ib]
}

func (it *mergeIter) Inc() {
	if it.lastA {
		it.ia++
	} else {
		it.ib++
	}

	it.lastA = it.nextA()
}

func (it *mergeIter) nextA() bool {
	if it.ia < len(it.a) && it.ib < len(it.b) && it.cmp(it.a[it.ia], it.b[it.ib]) <= 0 {
		return true
	} else if it.ia < len(it.a) && it.ib >= len(it.b) {
		return true
	}

	return false
}
