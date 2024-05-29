package defaultipam

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/internal/netiputil"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/libnetwork/ipbits"
	"github.com/docker/docker/libnetwork/types"
)

const (
	// DriverName is the name of the built-in default IPAM driver.
	DriverName = "default"

	localAddressSpace  = "LocalDefault"
	globalAddressSpace = "GlobalDefault"
)

// Register registers the default ipam driver with libnetwork. It takes
// two optional address pools respectively containing the list of user-defined
// address pools for 'local' and 'global' address spaces.
func Register(ic ipamapi.Registerer, lAddrPools, gAddrPools []*ipamutils.NetworkToSplit) error {
	if len(gAddrPools) == 0 {
		gAddrPools = ipamutils.GetGlobalScopeDefaultNetworks()
	}

	a, err := NewAllocator(lAddrPools, gAddrPools)
	if err != nil {
		return err
	}

	cps := &ipamapi.Capability{RequiresRequestReplay: true}

	return ic.RegisterIpamDriverWithCapabilities(DriverName, a, cps)
}

// Allocator provides per address space ipv4/ipv6 bookkeeping
type Allocator struct {
	// The address spaces
	local4, local6, global4, global6 *addrSpace
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(lcAs, glAs []*ipamutils.NetworkToSplit) (*Allocator, error) {
	var (
		a                          Allocator
		err                        error
		lcAs4, lcAs6, glAs4, glAs6 []*ipamutils.NetworkToSplit
	)

	lcAs4, lcAs6, err = splitByIPFamily(lcAs)
	if err != nil {
		return nil, fmt.Errorf("could not construct local address space: %w", err)
	}

	glAs4, glAs6, err = splitByIPFamily(glAs)
	if err != nil {
		return nil, fmt.Errorf("could not construct global address space: %w", err)
	}

	a.local4, err = newAddrSpace(lcAs4)
	if err != nil {
		return nil, fmt.Errorf("could not construct local v4 address space: %w", err)
	}
	a.local6, err = newAddrSpace(lcAs6)
	if err != nil {
		return nil, fmt.Errorf("could not construct local v6 address space: %w", err)
	}
	a.global4, err = newAddrSpace(glAs4)
	if err != nil {
		return nil, fmt.Errorf("could not construct global v4 address space: %w", err)
	}
	a.global6, err = newAddrSpace(glAs6)
	if err != nil {
		return nil, fmt.Errorf("could not construct global v6 address space: %w", err)
	}
	return &a, nil
}

func splitByIPFamily(s []*ipamutils.NetworkToSplit) ([]*ipamutils.NetworkToSplit, []*ipamutils.NetworkToSplit, error) {
	var v4, v6 []*ipamutils.NetworkToSplit

	for i, n := range s {
		if !n.Base.IsValid() || n.Size == 0 {
			return []*ipamutils.NetworkToSplit{}, []*ipamutils.NetworkToSplit{}, fmt.Errorf("network at index %d (%v) is not in canonical form", i, n)
		}
		if n.Base.Bits() > n.Size {
			return []*ipamutils.NetworkToSplit{}, []*ipamutils.NetworkToSplit{}, fmt.Errorf("network at index %d (%v) has a smaller prefix (/%d) than the target size of that pool (/%d)", i, n, n.Base.Bits(), n.Size)
		}

		n.Base, _ = n.Base.Addr().Unmap().Prefix(n.Base.Bits())

		if n.Base.Addr().Is4() {
			v4 = append(v4, n)
		} else {
			v6 = append(v6, n)
		}
	}

	return v4, v6, nil
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
func (a *Allocator) RequestPool(req ipamapi.PoolRequest) (ipamapi.AllocatedPool, error) {
	log.G(context.TODO()).Debugf("RequestPool: %+v", req)

	parseErr := func(err error) error {
		return types.InternalErrorf("failed to parse pool request for address space %q pool %q subpool %q: %v", req.AddressSpace, req.Pool, req.SubPool, err)
	}

	if req.AddressSpace == "" {
		return ipamapi.AllocatedPool{}, parseErr(ipamapi.ErrInvalidAddressSpace)
	}
	aSpace, err := a.getAddrSpace(req.AddressSpace, req.V6)
	if err != nil {
		return ipamapi.AllocatedPool{}, err
	}
	if req.Pool == "" && req.SubPool != "" {
		return ipamapi.AllocatedPool{}, parseErr(ipamapi.ErrInvalidSubPool)
	}

	k := PoolID{AddressSpace: req.AddressSpace}
	if req.Pool == "" {
		if k.Subnet, err = aSpace.allocatePredefinedPool(req.Exclude); err != nil {
			return ipamapi.AllocatedPool{}, err
		}
		return ipamapi.AllocatedPool{PoolID: k.String(), Pool: k.Subnet}, nil
	}

	if k.Subnet, err = netip.ParsePrefix(req.Pool); err != nil {
		return ipamapi.AllocatedPool{}, parseErr(ipamapi.ErrInvalidPool)
	}

	if req.SubPool != "" {
		if k.ChildSubnet, err = netip.ParsePrefix(req.SubPool); err != nil {
			return ipamapi.AllocatedPool{}, types.InternalErrorf("invalid pool request: %v", ipamapi.ErrInvalidSubPool)
		}
	}

	// This is a new non-master pool (subPool)
	if k.Subnet.IsValid() && k.ChildSubnet.IsValid() && k.Subnet.Addr().BitLen() != k.ChildSubnet.Addr().BitLen() {
		return ipamapi.AllocatedPool{}, types.InvalidParameterErrorf("pool and subpool are of incompatible address families")
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
		return ipamapi.AllocatedPool{}, types.ForbiddenErrorf("invalid pool request: %v", err)
	}

	return ipamapi.AllocatedPool{PoolID: k.String(), Pool: k.Subnet}, nil
}

// ReleasePool releases the address pool identified by the passed id
func (a *Allocator) ReleasePool(poolID string) error {
	log.G(context.TODO()).Debugf("ReleasePool(%s)", poolID)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace, k.Is6())
	if err != nil {
		return err
	}

	return aSpace.releaseSubnet(k.Subnet, k.ChildSubnet)
}

// Given the address space, returns the local or global PoolConfig based on whether the
// address space is local or global. AddressSpace locality is registered with IPAM out of band.
func (a *Allocator) getAddrSpace(as string, v6 bool) (*addrSpace, error) {
	switch as {
	case localAddressSpace:
		if v6 {
			return a.local6, nil
		}
		return a.local4, nil
	case globalAddressSpace:
		if v6 {
			return a.global6, nil
		}
		return a.global4, nil
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
	// enough to have one (i.e., anything bigger than a /31).
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

// RequestAddress returns an address from the specified pool ID
func (a *Allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	log.G(context.TODO()).Debugf("RequestAddress(%s, %v, %v)", poolID, prefAddress, opts)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return nil, nil, types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace, k.Is6())
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

// ReleaseAddress releases the address from the specified pool ID
func (a *Allocator) ReleaseAddress(poolID string, address net.IP) error {
	log.G(context.TODO()).Debugf("ReleaseAddress(%s, %v)", poolID, address)
	k, err := PoolIDFromString(poolID)
	if err != nil {
		return types.InvalidParameterErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace, k.Is6())
	if err != nil {
		return err
	}

	addr, ok := netip.AddrFromSlice(address)
	if !ok {
		return types.InvalidParameterErrorf("invalid address: %v", address)
	}

	return aSpace.releaseAddress(k.Subnet, k.ChildSubnet, addr.Unmap())
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
		ordinal = netiputil.HostID(prefAddress, uint(base.Bits()))
		err = bitmask.Set(ordinal)
	} else {
		start, end := netiputil.SubnetRange(base, ipr)
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

// IsBuiltIn returns true for builtin drivers
func (a *Allocator) IsBuiltIn() bool {
	return true
}
