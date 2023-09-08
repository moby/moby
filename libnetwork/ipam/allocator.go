package ipam

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipbits"
	"github.com/docker/docker/libnetwork/types"
)

const (
	localAddressSpace  = "LocalDefault"
	globalAddressSpace = "GlobalDefault"
)

// Allocator provides per address space ipv4/ipv6 book keeping
type Allocator struct {
	// The address spaces
	local, global *addrSpace
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(lcAs, glAs []*net.IPNet) (*Allocator, error) {
	var (
		a   Allocator
		err error
	)
	a.local, err = newAddrSpace(lcAs)
	if err != nil {
		return nil, fmt.Errorf("could not construct local address space: %w", err)
	}
	a.global, err = newAddrSpace(glAs)
	if err != nil {
		return nil, fmt.Errorf("could not construct global address space: %w", err)
	}
	return &a, nil
}

func newAddrSpace(predefined []*net.IPNet) (*addrSpace, error) {
	pdf := make([]netip.Prefix, len(predefined))
	for i, n := range predefined {
		var ok bool
		pdf[i], ok = toPrefix(n)
		if !ok {
			return nil, fmt.Errorf("network at index %d (%v) is not in canonical form", i, n)
		}
	}
	return &addrSpace{
		subnets:    map[netip.Prefix]*PoolData{},
		predefined: pdf,
	}, nil
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *Allocator) GetDefaultAddressSpaces() (string, string, error) {
	return localAddressSpace, globalAddressSpace, nil
}

// RequestPool returns an address pool along with its unique id.
// addressSpace must be a valid address space name and must not be the empty string.
// If requestedPool is the empty string then the default predefined pool for addressSpace will be used, otherwise pool must be a valid IP address and length in CIDR notation.
// If requestedSubPool is not empty, it must be a valid IP address and length in CIDR notation which is a sub-range of requestedPool.
// requestedSubPool must be empty if requestedPool is empty.
func (a *Allocator) RequestPool(addressSpace, requestedPool, requestedSubPool string, _ map[string]string, v6 bool) (poolID string, pool *net.IPNet, meta map[string]string, err error) {
	log.G(context.TODO()).Debugf("RequestPool(%s, %s, %s, _, %t)", addressSpace, requestedPool, requestedSubPool, v6)

	parseErr := func(err error) error {
		return types.InternalErrorf("failed to parse pool request for address space %q pool %q subpool %q: %v", addressSpace, requestedPool, requestedSubPool, err)
	}

	if addressSpace == "" {
		return "", nil, nil, parseErr(ipamapi.ErrInvalidAddressSpace)
	}
	aSpace, err := a.getAddrSpace(addressSpace)
	if err != nil {
		return "", nil, nil, err
	}
	if requestedPool == "" && requestedSubPool != "" {
		return "", nil, nil, parseErr(ipamapi.ErrInvalidSubPool)
	}

	k := PoolID{AddressSpace: addressSpace}
	if requestedPool == "" {
		k.Subnet, err = aSpace.allocatePredefinedPool(v6)
		if err != nil {
			return "", nil, nil, err
		}
		return k.String(), toIPNet(k.Subnet), nil, nil
	}

	if k.Subnet, err = netip.ParsePrefix(requestedPool); err != nil {
		return "", nil, nil, parseErr(ipamapi.ErrInvalidPool)
	}

	if requestedSubPool != "" {
		k.ChildSubnet, err = netip.ParsePrefix(requestedSubPool)
		if err != nil {
			return "", nil, nil, parseErr(ipamapi.ErrInvalidSubPool)
		}
	}

	k.Subnet, k.ChildSubnet = k.Subnet.Masked(), k.ChildSubnet.Masked()
	// Prior to https://github.com/moby/moby/pull/44968, libnetwork would happily accept a ChildSubnet with a bigger
	// mask than its parent subnet. In such case, it was producing IP addresses based on the parent subnet, and the
	// child subnet was not allocated from the address pool. Following condition take care of restoring this behavior
	// for networks created before upgrading to v24.0.
	if k.ChildSubnet.IsValid() && k.ChildSubnet.Bits() < k.Subnet.Bits() {
		k.ChildSubnet = k.Subnet
	}

	err = aSpace.allocateSubnet(k.Subnet, k.ChildSubnet)
	if err != nil {
		return "", nil, nil, err
	}

	return k.String(), toIPNet(k.Subnet), nil, nil
}

// ReleasePool releases the address pool identified by the passed id
func (a *Allocator) ReleasePool(poolID string) error {
	log.G(context.TODO()).Debugf("ReleasePool(%s)", poolID)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	return aSpace.releaseSubnet(k.Subnet, k.ChildSubnet)
}

// Given the address space, returns the local or global PoolConfig based on whether the
// address space is local or global. AddressSpace locality is registered with IPAM out of band.
func (a *Allocator) getAddrSpace(as string) (*addrSpace, error) {
	switch as {
	case localAddressSpace:
		return a.local, nil
	case globalAddressSpace:
		return a.global, nil
	}
	return nil, types.InvalidParameterErrorf("cannot find address space %s", as)
}

func newPoolData(pool netip.Prefix) *PoolData {
	ones, bits := pool.Bits(), pool.Addr().BitLen()
	numAddresses := uint64(1 << uint(bits-ones))

	// Allow /64 subnet
	if pool.Addr().Is6() && numAddresses == 0 {
		numAddresses--
	}

	// Generate the new address masks.
	h := bitmap.New(numAddresses)

	// Pre-reserve the network address on IPv4 networks large
	// enough to have one (i.e., anything bigger than a /31.
	if !(pool.Addr().Is4() && numAddresses <= 2) {
		h.Set(0)
	}

	// Pre-reserve the broadcast address on IPv4 networks large
	// enough to have one (i.e., anything bigger than a /31).
	if pool.Addr().Is4() && numAddresses > 2 {
		h.Set(numAddresses - 1)
	}

	return &PoolData{addrs: h, children: map[netip.Prefix]struct{}{}}
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
	aSpace.Lock()
	defer aSpace.Unlock()

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
		if !aSpace.contains(nw) {
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

// RequestAddress returns an address from the specified pool ID
func (a *Allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	log.G(context.TODO()).Debugf("RequestAddress(%s, %v, %v)", poolID, prefAddress, opts)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return nil, nil, types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return nil, nil, err
	}
	var pref netip.Addr
	if prefAddress != nil {
		var ok bool
		pref, ok = netip.AddrFromSlice(prefAddress)
		if !ok {
			return nil, nil, types.InvalidParameterErrorf("invalid preferred address: %v", prefAddress)
		}
	}
	p, err := aSpace.requestAddress(k.Subnet, k.ChildSubnet, pref.Unmap(), opts)
	if err != nil {
		return nil, nil, err
	}
	return &net.IPNet{
		IP:   p.AsSlice(),
		Mask: net.CIDRMask(k.Subnet.Bits(), k.Subnet.Addr().BitLen()),
	}, nil, nil
}

func (aSpace *addrSpace) requestAddress(nw, sub netip.Prefix, prefAddress netip.Addr, opts map[string]string) (netip.Addr, error) {
	aSpace.Lock()
	defer aSpace.Unlock()

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

// ReleaseAddress releases the address from the specified pool ID
func (a *Allocator) ReleaseAddress(poolID string, address net.IP) error {
	log.G(context.TODO()).Debugf("ReleaseAddress(%s, %v)", poolID, address)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	addr, ok := netip.AddrFromSlice(address)
	if !ok {
		return types.InvalidParameterErrorf("invalid address: %v", address)
	}

	return aSpace.releaseAddress(k.Subnet, k.ChildSubnet, addr.Unmap())
}

func (aSpace *addrSpace) releaseAddress(nw, sub netip.Prefix, address netip.Addr) error {
	aSpace.Lock()
	defer aSpace.Unlock()

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

	return p.addrs.Unset(hostID(address, uint(nw.Bits())))
}

func getAddress(base netip.Prefix, bitmask *bitmap.Bitmap, prefAddress netip.Addr, ipr netip.Prefix, serial bool) (netip.Addr, error) {
	var (
		ordinal uint64
		err     error
	)

	log.G(context.TODO()).Debugf("Request address PoolID:%v %s Serial:%v PrefAddress:%v ", base, bitmask, serial, prefAddress)

	if bitmask.Unselected() == 0 {
		return netip.Addr{}, ipamapi.ErrNoAvailableIPs
	}
	if ipr == (netip.Prefix{}) && prefAddress == (netip.Addr{}) {
		ordinal, err = bitmask.SetAny(serial)
	} else if prefAddress != (netip.Addr{}) {
		ordinal = hostID(prefAddress, uint(base.Bits()))
		err = bitmask.Set(ordinal)
	} else {
		start, end := subnetRange(base, ipr)
		ordinal, err = bitmask.SetAnyInRange(start, end, serial)
	}

	switch err {
	case nil:
		// Convert IP ordinal for this subnet into IP address
		return ipbits.Add(base.Addr(), ordinal, 0), nil
	case bitmap.ErrBitAllocated:
		return netip.Addr{}, ipamapi.ErrIPAlreadyAllocated
	case bitmap.ErrNoBitAvailable:
		return netip.Addr{}, ipamapi.ErrNoAvailableIPs
	default:
		return netip.Addr{}, err
	}
}

// DumpDatabase dumps the internal info
func (a *Allocator) DumpDatabase() string {
	aspaces := map[string]*addrSpace{
		localAddressSpace:  a.local,
		globalAddressSpace: a.global,
	}

	var b strings.Builder
	for _, as := range []string{localAddressSpace, globalAddressSpace} {
		fmt.Fprintf(&b, "\n### %s\n", as)
		b.WriteString(aspaces[as].DumpDatabase())
	}
	return b.String()
}

func (aSpace *addrSpace) DumpDatabase() string {
	aSpace.Lock()
	defer aSpace.Unlock()

	var b strings.Builder
	for k, config := range aSpace.subnets {
		fmt.Fprintf(&b, "%v: %v\n", k, config)
		fmt.Fprintf(&b, "  Bitmap: %v\n", config.addrs)
		for k := range config.children {
			fmt.Fprintf(&b, "  - Subpool: %v\n", k)
		}
	}
	return b.String()
}

// IsBuiltIn returns true for builtin drivers
func (a *Allocator) IsBuiltIn() bool {
	return true
}
