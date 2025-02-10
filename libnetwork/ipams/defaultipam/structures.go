package defaultipam

import (
	"fmt"
	"math"
	"net/netip"
	"strings"

	"github.com/docker/docker/libnetwork/internal/addrset"
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
	addrs    *addrset.AddrSet
	children map[netip.Prefix]struct{}

	usedRange uint64

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

func (p *PoolData) RequestAddress(nw, sub netip.Prefix, prefAddress netip.Addr, opts map[string]string) (netip.Addr, error) {

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

	if len(p.children) == 0 {
		p.incUsedAddrsRange()
	} else {
		for ch := range p.children {
			if ch.Contains(ip) {
				p.incUsedAddrsRange()
			}
		}
	}
	return ip, nil
}

func (p *PoolData) ReleaseAddress(nw, sub netip.Prefix, address netip.Addr) error {
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

	if err := p.addrs.Remove(address); err != nil {
		return err
	}

	if len(p.children) == 0 {
		p.decUsedAddrsRange(nw)
	} else {
		for sub := range p.children {
			if sub.Contains(address) {
				p.decUsedAddrsRange(nw)
			}
		}
	}
	return nil
}

func (p *PoolData) incUsedAddrsRange() {
	if p.usedRange < math.MaxUint64 {
		p.usedRange += 1
	}
}

func (p *PoolData) decUsedAddrsRange(nw netip.Prefix) {
	if p.usedRange > 0 {
		p.usedRange -= 1
	}
}

func (p *PoolData) UsedAddrs() (usedSubnet uint64, usedRange uint64) {
	return p.addrs.Selected(), p.usedRange
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
