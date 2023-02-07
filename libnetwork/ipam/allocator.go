package ipam

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
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
	return &Allocator{
		local:  newAddrSpace(lcAs),
		global: newAddrSpace(glAs),
	}, nil
}

func newAddrSpace(predefined []*net.IPNet) *addrSpace {
	return &addrSpace{
		subnets:    map[string]*PoolData{},
		predefined: predefined,
	}
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *Allocator) GetDefaultAddressSpaces() (string, string, error) {
	return localAddressSpace, globalAddressSpace, nil
}

// RequestPool returns an address pool along with its unique id.
// addressSpace must be a valid address space name and must not be the empty string.
// If pool is the empty string then the default predefined pool for addressSpace will be used, otherwise pool must be a valid IP address and length in CIDR notation.
// If subPool is not empty, it must be a valid IP address and length in CIDR notation which is a sub-range of pool.
// subPool must be empty if pool is empty.
func (a *Allocator) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	logrus.Debugf("RequestPool(%s, %s, %s, %v, %t)", addressSpace, pool, subPool, options, v6)

	parseErr := func(err error) (string, *net.IPNet, map[string]string, error) {
		return "", nil, nil, types.InternalErrorf("failed to parse pool request for address space %q pool %q subpool %q: %v", addressSpace, pool, subPool, err)
	}

	if addressSpace == "" {
		return parseErr(ipamapi.ErrInvalidAddressSpace)
	}
	aSpace, err := a.getAddrSpace(addressSpace)
	if err != nil {
		return "", nil, nil, err
	}
	k := PoolID{AddressSpace: addressSpace}

	if pool == "" {
		if subPool != "" {
			return parseErr(ipamapi.ErrInvalidSubPool)
		}
		var nw *net.IPNet
		nw, k.SubnetKey, err = aSpace.allocatePredefinedPool(v6)
		if err != nil {
			return "", nil, nil, err
		}
		return k.String(), nw, nil, nil
	}

	var (
		nw, sub *net.IPNet
	)
	if _, nw, err = net.ParseCIDR(pool); err != nil {
		return parseErr(ipamapi.ErrInvalidPool)
	}

	if subPool != "" {
		var err error
		_, sub, err = net.ParseCIDR(subPool)
		if err != nil {
			return parseErr(ipamapi.ErrInvalidSubPool)
		}
		k.ChildSubnet = subPool
	}

	k.SubnetKey, err = aSpace.allocateSubnet(nw, sub)
	if err != nil {
		return "", nil, nil, err
	}

	return k.String(), nw, nil, nil
}

// ReleasePool releases the address pool identified by the passed id
func (a *Allocator) ReleasePool(poolID string) error {
	logrus.Debugf("ReleasePool(%s)", poolID)
	k := PoolID{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	return aSpace.releaseSubnet(k.SubnetKey)
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
	return nil, types.BadRequestErrorf("cannot find address space %s", as)
}

func newPoolData(pool *net.IPNet) *PoolData {
	ipVer := getAddressVersion(pool.IP)
	ones, bits := pool.Mask.Size()
	numAddresses := uint64(1 << uint(bits-ones))

	// Allow /64 subnet
	if ipVer == v6 && numAddresses == 0 {
		numAddresses--
	}

	// Generate the new address masks.
	h := bitmap.New(numAddresses)

	// Pre-reserve the network address on IPv4 networks large
	// enough to have one (i.e., anything bigger than a /31.
	if !(ipVer == v4 && numAddresses <= 2) {
		h.Set(0)
	}

	// Pre-reserve the broadcast address on IPv4 networks large
	// enough to have one (i.e., anything bigger than a /31).
	if ipVer == v4 && numAddresses > 2 {
		h.Set(numAddresses - 1)
	}

	return &PoolData{Pool: pool, addrs: h, children: map[string]struct{}{}}
}

// getPredefineds returns the predefined subnets for the address space.
//
// It should not be called concurrently with any other method on the addrSpace.
func (aSpace *addrSpace) getPredefineds() []*net.IPNet {
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

func (aSpace *addrSpace) allocatePredefinedPool(ipV6 bool) (*net.IPNet, SubnetKey, error) {
	var v ipVersion
	v = v4
	if ipV6 {
		v = v6
	}

	aSpace.Lock()
	defer aSpace.Unlock()

	for i, nw := range aSpace.getPredefineds() {
		if v != getAddressVersion(nw.IP) {
			continue
		}
		// Checks whether pool has already been allocated
		if _, ok := aSpace.subnets[nw.String()]; ok {
			continue
		}
		// Shouldn't be necessary, but check prevents IP collisions should
		// predefined pools overlap for any reason.
		if !aSpace.contains(nw) {
			aSpace.updatePredefinedStartIndex(i + 1)
			k, err := aSpace.allocateSubnetL(nw, nil)
			if err != nil {
				return nil, SubnetKey{}, err
			}
			return nw, k, nil
		}
	}

	return nil, SubnetKey{}, types.NotFoundErrorf("could not find an available, non-overlapping IPv%d address pool among the defaults to assign to the network", v)
}

// RequestAddress returns an address from the specified pool ID
func (a *Allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	logrus.Debugf("RequestAddress(%s, %v, %v)", poolID, prefAddress, opts)
	k := PoolID{}
	if err := k.FromString(poolID); err != nil {
		return nil, nil, types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return nil, nil, err
	}
	return aSpace.requestAddress(k.SubnetKey, prefAddress, opts)
}

func (aSpace *addrSpace) requestAddress(k SubnetKey, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	aSpace.Lock()
	defer aSpace.Unlock()

	p, ok := aSpace.subnets[k.Subnet]
	if !ok {
		return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%+v", k)
	}

	if prefAddress != nil && !p.Pool.Contains(prefAddress) {
		return nil, nil, ipamapi.ErrIPOutOfRange
	}

	var ipr *AddressRange
	if k.ChildSubnet != "" {
		if _, ok := p.children[k.ChildSubnet]; !ok {
			return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%+v", k)
		}
		_, sub, err := net.ParseCIDR(k.ChildSubnet)
		if err != nil {
			return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%+v: %v", k, err)
		}
		ipr, err = getAddressRange(sub, p.Pool)
		if err != nil {
			return nil, nil, err
		}
	}

	// In order to request for a serial ip address allocation, callers can pass in the option to request
	// IP allocation serially or first available IP in the subnet
	serial := opts[ipamapi.AllocSerialPrefix] == "true"
	ip, err := getAddress(p.Pool, p.addrs, prefAddress, ipr, serial)
	if err != nil {
		return nil, nil, err
	}

	return &net.IPNet{IP: ip, Mask: p.Pool.Mask}, nil, nil
}

// ReleaseAddress releases the address from the specified pool ID
func (a *Allocator) ReleaseAddress(poolID string, address net.IP) error {
	logrus.Debugf("ReleaseAddress(%s, %v)", poolID, address)
	k := PoolID{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	return aSpace.releaseAddress(k.SubnetKey, address)
}

func (aSpace *addrSpace) releaseAddress(k SubnetKey, address net.IP) error {
	aSpace.Lock()
	defer aSpace.Unlock()

	p, ok := aSpace.subnets[k.Subnet]
	if !ok {
		return types.NotFoundErrorf("cannot find address pool for %+v", k)
	}
	if k.ChildSubnet != "" {
		if _, ok := p.children[k.ChildSubnet]; !ok {
			return types.NotFoundErrorf("cannot find address pool for poolID:%+v", k)
		}
	}

	if address == nil {
		return types.BadRequestErrorf("invalid address: nil")
	}

	if !p.Pool.Contains(address) {
		return ipamapi.ErrIPOutOfRange
	}

	mask := p.Pool.Mask

	h, err := types.GetHostPartIP(address, mask)
	if err != nil {
		return types.InternalErrorf("failed to release address %s: %v", address, err)
	}

	defer logrus.Debugf("Released address Address:%v Sequence:%s", address, p.addrs)

	return p.addrs.Unset(ipToUint64(h))
}

func getAddress(nw *net.IPNet, bitmask *bitmap.Bitmap, prefAddress net.IP, ipr *AddressRange, serial bool) (net.IP, error) {
	var (
		ordinal uint64
		err     error
		base    *net.IPNet
	)

	logrus.Debugf("Request address PoolID:%v %s Serial:%v PrefAddress:%v ", nw, bitmask, serial, prefAddress)
	base = types.GetIPNetCopy(nw)

	if bitmask.Unselected() == 0 {
		return nil, ipamapi.ErrNoAvailableIPs
	}
	if ipr == nil && prefAddress == nil {
		ordinal, err = bitmask.SetAny(serial)
	} else if prefAddress != nil {
		hostPart, e := types.GetHostPartIP(prefAddress, base.Mask)
		if e != nil {
			return nil, types.InternalErrorf("failed to allocate requested address %s: %v", prefAddress.String(), e)
		}
		ordinal = ipToUint64(types.GetMinimalIP(hostPart))
		err = bitmask.Set(ordinal)
	} else {
		ordinal, err = bitmask.SetAnyInRange(ipr.Start, ipr.End, serial)
	}

	switch err {
	case nil:
		// Convert IP ordinal for this subnet into IP address
		return generateAddress(ordinal, base), nil
	case bitmap.ErrBitAllocated:
		return nil, ipamapi.ErrIPAlreadyAllocated
	case bitmap.ErrNoBitAvailable:
		return nil, ipamapi.ErrNoAvailableIPs
	default:
		return nil, err
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
