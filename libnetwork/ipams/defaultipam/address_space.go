// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package defaultipam

import (
	"context"
	"net/netip"
	"slices"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/ipbits"
	"github.com/docker/docker/libnetwork/types"
)

// addrSpace contains the pool configurations for the address space
type addrSpace struct {
	// Ordered list of allocated subnets. This field is used for dynamic subnet
	// allocations.
	allocated []netip.Prefix
	// Allocated subnets, indexed by their prefix. Values track address
	// allocations.
	subnets map[netip.Prefix]*PoolData

	// predefined pools for the address space
	predefined []*ipamutils.NetworkToSplit

	mu sync.Mutex
}

func newAddrSpace(predefined []*ipamutils.NetworkToSplit) (*addrSpace, error) {
	slices.SortFunc(predefined, func(a, b *ipamutils.NetworkToSplit) int {
		return netiputil.PrefixCompare(a.Base, b.Base)
	})

	// We need to discard longer overlapping prefixes (sorted after the shorter
	// one), otherwise the dynamic allocator might consider a predefined
	// network is fully overlapped, go to the next one, which is a subnet of
	// the previous, and allocate from it.
	j := 0
	for i := 1; i < len(predefined); i++ {
		if predefined[j].Overlaps(predefined[i].Base) {
			continue
		}
		j++
		predefined[j] = predefined[i]
	}

	if len(predefined) > j {
		j++
	}
	clear(predefined[j:])

	return &addrSpace{
		subnets:    map[netip.Prefix]*PoolData{},
		predefined: predefined[:j],
	}, nil
}

// allocateSubnet makes a static allocation for subnets 'nw' and 'sub'.
//
// This method is safe for concurrent use.
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

// allocateSubnetL takes a 'nw' parent prefix and a 'sub' prefix. These are
// '--subnet' and '--ip-range' on the CLI.
//
// If 'sub' prefix is specified, we don't check if 'parent' overlaps with
// existing allocations. However, if no 'sub' prefix is specified, we do check
// for overlaps. This behavior is weird and leads to the inconsistencies
// documented in https://github.com/moby/moby/issues/46756.
func (aSpace *addrSpace) allocateSubnetL(nw, sub netip.Prefix) error {
	// If master pool, check for overlap
	if sub == (netip.Prefix{}) {
		if aSpace.overlaps(nw) {
			return ipamapi.ErrPoolOverlap
		}
		return aSpace.allocatePool(nw)
	}

	// Look for parent pool
	_, ok := aSpace.subnets[nw]
	if !ok {
		if err := aSpace.allocatePool(nw); err != nil {
			return err
		}
		aSpace.subnets[nw].autoRelease = true
	}
	aSpace.subnets[nw].children[sub] = struct{}{}
	return nil
}

// overlaps reports whether nw contains any IP addresses in common with any of
// the existing subnets in this address space.
func (aSpace *addrSpace) overlaps(nw netip.Prefix) bool {
	for _, allocated := range aSpace.allocated {
		if allocated.Overlaps(nw) {
			return true
		}
	}
	return false
}

func (aSpace *addrSpace) allocatePool(nw netip.Prefix) error {
	n, _ := slices.BinarySearchFunc(aSpace.allocated, nw, netiputil.PrefixCompare)
	aSpace.allocated = slices.Insert(aSpace.allocated, n, nw)
	aSpace.subnets[nw] = newPoolData(nw)
	return nil
}

// allocatePredefinedPool dynamically allocates a subnet that doesn't overlap
// with existing allocations and 'reserved' prefixes.
//
// This method is safe for concurrent use.
func (aSpace *addrSpace) allocatePredefinedPool(reserved []netip.Prefix) (netip.Prefix, error) {
	aSpace.mu.Lock()
	defer aSpace.mu.Unlock()

	var pdfID int
	var partialOverlap bool
	var prevAlloc netip.Prefix

	it := newMergeIter(aSpace.allocated, reserved, netiputil.PrefixCompare)

	makeAlloc := func(subnet netip.Prefix) netip.Prefix {
		// it.ia tracks the position of the mergeIter within aSpace.allocated.
		aSpace.allocated = slices.Insert(aSpace.allocated, it.ia, subnet)
		aSpace.subnets[subnet] = newPoolData(subnet)
		return subnet
	}

	for {
		allocated := it.Get()
		if allocated == (netip.Prefix{}) {
			// We reached the end of both 'aSpace.allocated' and 'reserved'.
			break
		}

		if pdfID >= len(aSpace.predefined) {
			return netip.Prefix{}, ipamapi.ErrNoMoreSubnets
		}
		pdf := aSpace.predefined[pdfID]

		if allocated.Overlaps(pdf.Base) {
			if allocated.Bits() <= pdf.Base.Bits() {
				// The current 'allocated' prefix is bigger than the 'pdf'
				// network, thus the block is fully overlapped.
				partialOverlap = false
				prevAlloc = netip.Prefix{}
				pdfID++
				continue
			}

			// If no previous 'allocated' was found to partially overlap 'pdf',
			// we need to test whether there's enough space available at the
			// beginning of 'pdf'.
			if !partialOverlap && ipbits.SubnetsBetween(pdf.FirstPrefix().Addr(), allocated.Addr(), pdf.Size) >= 1 {
				// Okay, so there's at least a whole subnet available between
				// the start of 'pdf' and 'allocated'.
				next := pdf.FirstPrefix()
				return makeAlloc(next), nil
			}

			// If the network 'pdf' was already found to be partially
			// overlapped, we need to test whether there's enough space between
			// the end of 'prevAlloc' and current 'allocated'.
			afterPrev := netiputil.PrefixAfter(prevAlloc, pdf.Size)
			if partialOverlap && ipbits.SubnetsBetween(afterPrev.Addr(), allocated.Addr(), pdf.Size) >= 1 {
				// Okay, so there's at least a whole subnet available after
				// 'prevAlloc' and before 'allocated'.
				return makeAlloc(afterPrev), nil
			}

			it.Inc()

			if netiputil.LastAddr(allocated) == netiputil.LastAddr(pdf.Base) {
				// The last address of the current 'allocated' prefix is the
				// same as the last address of the 'pdf' network, it's fully
				// overlapped.
				partialOverlap = false
				prevAlloc = netip.Prefix{}
				pdfID++
				continue
			}

			// This 'pdf' network is partially overlapped.
			partialOverlap = true
			prevAlloc = allocated
			continue
		}

		// Okay, so previous 'allocated' overlapped and current doesn't. Now
		// the question is: is there enough space left between previous
		// 'allocated' and the end of the 'pdf' network?
		if partialOverlap {
			partialOverlap = false

			if next := netiputil.PrefixAfter(prevAlloc, pdf.Size); pdf.Overlaps(next) {
				return makeAlloc(next), nil
			}

			// No luck, PrefixAfter yielded an invalid prefix. There's not
			// enough space left to subnet it once more.
			pdfID++

			// 'it' is not incremented here, we need to re-test the current
			// 'allocated' against the next 'pdf' network.
			continue
		}

		// If the network 'pdf' doesn't overlap and is sorted before the
		// current 'allocated', we found the right spot.
		if pdf.Base.Addr().Less(allocated.Addr()) {
			next := netip.PrefixFrom(pdf.Base.Addr(), pdf.Size)
			return makeAlloc(next), nil
		}

		it.Inc()
		prevAlloc = allocated
	}

	if pdfID >= len(aSpace.predefined) {
		return netip.Prefix{}, ipamapi.ErrNoMoreSubnets
	}

	// We reached the end of 'allocated', but not the end of predefined
	// networks. Let's try two more times (once on the current 'pdf', and once
	// on the next network if any).
	if partialOverlap {
		pdf := aSpace.predefined[pdfID]

		if next := netiputil.PrefixAfter(prevAlloc, pdf.Size); pdf.Overlaps(next) {
			return makeAlloc(next), nil
		}

		// No luck -- PrefixAfter yielded an invalid prefix. There's not enough
		// space left.
		pdfID++
	}

	// One last chance. Here we don't increment pdfID since the last iteration
	// on 'it' found either:
	//
	// - A full overlap, and incremented 'pdfID'.
	// - A partial overlap, and the previous 'if' incremented 'pdfID'.
	// - The current 'pdfID' comes after the last 'allocated' -- it's not
	//   overlapped at all.
	//
	// Hence, we're sure 'pdfID' has never been subnetted yet.
	if pdfID < len(aSpace.predefined) {
		pdf := aSpace.predefined[pdfID]

		next := pdf.FirstPrefix()
		return makeAlloc(next), nil
	}

	return netip.Prefix{}, ipamapi.ErrNoMoreSubnets
}

// releaseSubnet deallocates prefixes nw and sub. It returns an error if no
// matching allocations could be found.
//
// This method is safe for concurrent use.
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
		aSpace.deallocate(nw)
	}

	return nil
}

// deallocate removes 'nw' from the list of allocations.
func (aSpace *addrSpace) deallocate(nw netip.Prefix) {
	if i, ok := slices.BinarySearchFunc(aSpace.allocated, nw, netiputil.PrefixCompare); ok {
		aSpace.allocated = slices.Delete(aSpace.allocated, i, i+1)
		delete(aSpace.subnets, nw)
	}
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
