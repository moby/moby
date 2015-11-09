package ipam

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/ipamutils"
	"github.com/docker/libnetwork/types"
)

const (
	localAddressSpace  = "LocalDefault"
	globalAddressSpace = "GlobalDefault"
	// The biggest configurable host subnets
	minNetSize   = 8
	minNetSizeV6 = 64
	// datastore keyes for ipam objects
	dsConfigKey = "ipam/" + ipamapi.DefaultIPAM + "/config"
	dsDataKey   = "ipam/" + ipamapi.DefaultIPAM + "/data"
)

// Allocator provides per address space ipv4/ipv6 book keeping
type Allocator struct {
	// Predefined pools for default address spaces
	predefined map[string][]*net.IPNet
	addrSpaces map[string]*addrSpace
	// stores        []datastore.Datastore
	// Allocated addresses in each address space's subnet
	addresses map[SubnetKey]*bitseq.Handle
	sync.Mutex
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(lcDs, glDs datastore.DataStore) (*Allocator, error) {
	a := &Allocator{}

	// Load predefined subnet pools
	a.predefined = map[string][]*net.IPNet{
		localAddressSpace:  ipamutils.PredefinedBroadNetworks,
		globalAddressSpace: ipamutils.PredefinedGranularNetworks,
	}

	// Initialize bitseq map
	a.addresses = make(map[SubnetKey]*bitseq.Handle)

	// Initialize address spaces
	a.addrSpaces = make(map[string]*addrSpace)
	for _, aspc := range []struct {
		as string
		ds datastore.DataStore
	}{
		{localAddressSpace, lcDs},
		{globalAddressSpace, glDs},
	} {
		if aspc.ds == nil {
			continue
		}

		a.addrSpaces[aspc.as] = &addrSpace{
			subnets: map[SubnetKey]*PoolData{},
			id:      dsConfigKey + "/" + aspc.as,
			scope:   aspc.ds.Scope(),
			ds:      aspc.ds,
			alloc:   a,
		}
	}

	return a, nil
}

func (a *Allocator) refresh(as string) error {
	aSpace, err := a.getAddressSpaceFromStore(as)
	if err != nil {
		return fmt.Errorf("error getting pools config from store during init: %v",
			err)
	}

	if aSpace == nil {
		return nil
	}

	a.Lock()
	a.addrSpaces[as] = aSpace
	a.Unlock()

	return nil
}

func (a *Allocator) updateBitMasks(aSpace *addrSpace) error {
	var inserterList []func() error

	aSpace.Lock()
	for k, v := range aSpace.subnets {
		if v.Range == nil {
			kk := k
			vv := v
			inserterList = append(inserterList, func() error { return a.insertBitMask(kk, vv.Pool) })
		}
	}
	aSpace.Unlock()

	// Add the bitmasks (data could come from datastore)
	if inserterList != nil {
		for _, f := range inserterList {
			if err := f(); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *Allocator) GetDefaultAddressSpaces() (string, string, error) {
	return localAddressSpace, globalAddressSpace, nil
}

// RequestPool returns an address pool along with its unique id.
func (a *Allocator) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	log.Debugf("RequestPool(%s, %s, %s, %v, %t)", addressSpace, pool, subPool, options, v6)
	k, nw, ipr, err := a.parsePoolRequest(addressSpace, pool, subPool, v6)
	if err != nil {
		return "", nil, nil, types.InternalErrorf("failed to parse pool request for address space %q pool %q subpool %q: %v", addressSpace, pool, subPool, err)
	}

retry:
	if err := a.refresh(addressSpace); err != nil {
		return "", nil, nil, err
	}

	aSpace, err := a.getAddrSpace(addressSpace)
	if err != nil {
		return "", nil, nil, err
	}

	insert, err := aSpace.updatePoolDBOnAdd(*k, nw, ipr)
	if err != nil {
		return "", nil, nil, err
	}

	if err := a.writeToStore(aSpace); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return "", nil, nil, types.InternalErrorf("pool configuration failed because of %s", err.Error())
		}

		goto retry
	}

	return k.String(), nw, nil, insert()
}

// ReleasePool releases the address pool identified by the passed id
func (a *Allocator) ReleasePool(poolID string) error {
	log.Debugf("ReleasePool(%s)", poolID)
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

retry:
	if err := a.refresh(k.AddressSpace); err != nil {
		return err
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	remove, err := aSpace.updatePoolDBOnRemoval(k)
	if err != nil {
		return err
	}

	if err = a.writeToStore(aSpace); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return types.InternalErrorf("pool (%s) removal failed because of %v", poolID, err)
		}
		goto retry
	}

	return remove()
}

// Given the address space, returns the local or global PoolConfig based on the
// address space is local or global. AddressSpace locality is being registered with IPAM out of band.
func (a *Allocator) getAddrSpace(as string) (*addrSpace, error) {
	a.Lock()
	defer a.Unlock()
	aSpace, ok := a.addrSpaces[as]
	if !ok {
		return nil, types.BadRequestErrorf("cannot find address space %s (most likely the backing datastore is not configured)", as)
	}
	return aSpace, nil
}

func (a *Allocator) parsePoolRequest(addressSpace, pool, subPool string, v6 bool) (*SubnetKey, *net.IPNet, *AddressRange, error) {
	var (
		nw  *net.IPNet
		ipr *AddressRange
		err error
	)

	if addressSpace == "" {
		return nil, nil, nil, ipamapi.ErrInvalidAddressSpace
	}

	if pool == "" && subPool != "" {
		return nil, nil, nil, ipamapi.ErrInvalidSubPool
	}

	if pool != "" {
		if _, nw, err = net.ParseCIDR(pool); err != nil {
			return nil, nil, nil, ipamapi.ErrInvalidPool
		}
		if subPool != "" {
			if ipr, err = getAddressRange(subPool, nw); err != nil {
				return nil, nil, nil, err
			}
		}
	} else {
		if nw, err = a.getPredefinedPool(addressSpace, v6); err != nil {
			return nil, nil, nil, err
		}

	}

	return &SubnetKey{AddressSpace: addressSpace, Subnet: nw.String(), ChildSubnet: subPool}, nw, ipr, nil
}

func (a *Allocator) insertBitMask(key SubnetKey, pool *net.IPNet) error {
	//log.Debugf("Inserting bitmask (%s, %s)", key.String(), pool.String())

	store := a.getStore(key.AddressSpace)
	if store == nil {
		return fmt.Errorf("could not find store for address space %s while inserting bit mask", key.AddressSpace)
	}

	ipVer := getAddressVersion(pool.IP)
	ones, bits := pool.Mask.Size()
	numAddresses := uint64(1 << uint(bits-ones))

	// Allow /64 subnet
	if ipVer == v6 && numAddresses == 0 {
		numAddresses--
	}

	// Generate the new address masks. AddressMask content may come from datastore
	h, err := bitseq.NewHandle(dsDataKey, store, key.String(), numAddresses)
	if err != nil {
		return err
	}

	// Do not let network identifier address be reserved
	// Do the same for IPv6 so that bridge ip starts with XXXX...::1
	h.Set(0)

	// Do not let broadcast address be reserved
	if ipVer == v4 {
		h.Set(numAddresses - 1)
	}

	a.Lock()
	a.addresses[key] = h
	a.Unlock()
	return nil
}

func (a *Allocator) retrieveBitmask(k SubnetKey, n *net.IPNet) (*bitseq.Handle, error) {
	a.Lock()
	bm, ok := a.addresses[k]
	a.Unlock()
	if !ok {
		log.Debugf("Retrieving bitmask (%s, %s)", k.String(), n.String())
		if err := a.insertBitMask(k, n); err != nil {
			return nil, fmt.Errorf("could not find bitmask in datastore for %s", k.String())
		}
		a.Lock()
		bm = a.addresses[k]
		a.Unlock()
	}
	return bm, nil
}

func (a *Allocator) getPredefineds(as string) []*net.IPNet {
	a.Lock()
	defer a.Unlock()
	l := make([]*net.IPNet, 0, len(a.predefined[as]))
	for _, pool := range a.predefined[as] {
		l = append(l, pool)
	}
	return l
}

func (a *Allocator) getPredefinedPool(as string, ipV6 bool) (*net.IPNet, error) {
	var v ipVersion
	v = v4
	if ipV6 {
		v = v6
	}

	if as != localAddressSpace && as != globalAddressSpace {
		return nil, fmt.Errorf("no default pool availbale for non-default addresss spaces")
	}

	aSpace, err := a.getAddrSpace(as)
	if err != nil {
		return nil, err
	}

	for _, nw := range a.getPredefineds(as) {
		if v != getAddressVersion(nw.IP) {
			continue
		}
		aSpace.Lock()
		_, ok := aSpace.subnets[SubnetKey{AddressSpace: as, Subnet: nw.String()}]
		aSpace.Unlock()
		if ok {
			continue
		}

		if !aSpace.contains(as, nw) {
			if as == localAddressSpace {
				// Check if nw overlap with system routes, name servers
				if _, err := ipamutils.FindAvailableNetwork([]*net.IPNet{nw}); err == nil {
					return nw, nil
				}
				continue
			}
			return nw, nil
		}
	}

	return nil, types.NotFoundErrorf("could not find an available predefined network")
}

// RequestAddress returns an address from the specified pool ID
func (a *Allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	log.Debugf("RequestAddress(%s, %v, %v)", poolID, prefAddress, opts)
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return nil, nil, types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	if err := a.refresh(k.AddressSpace); err != nil {
		return nil, nil, err
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return nil, nil, err
	}

	aSpace.Lock()
	p, ok := aSpace.subnets[k]
	if !ok {
		aSpace.Unlock()
		return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%s", poolID)
	}

	if prefAddress != nil && !p.Pool.Contains(prefAddress) {
		aSpace.Unlock()
		return nil, nil, ipamapi.ErrIPOutOfRange
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c, ok = aSpace.subnets[k]
	}
	aSpace.Unlock()

	bm, err := a.retrieveBitmask(k, c.Pool)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find bitmask in datastore for %s on address %v request from pool %s: %v",
			k.String(), prefAddress, poolID, err)
	}
	ip, err := a.getAddress(p.Pool, bm, prefAddress, p.Range)
	if err != nil {
		return nil, nil, err
	}

	return &net.IPNet{IP: ip, Mask: p.Pool.Mask}, nil, nil
}

// ReleaseAddress releases the address from the specified pool ID
func (a *Allocator) ReleaseAddress(poolID string, address net.IP) error {
	log.Debugf("ReleaseAddress(%s, %v)", poolID, address)
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	if err := a.refresh(k.AddressSpace); err != nil {
		return err
	}

	aSpace, err := a.getAddrSpace(k.AddressSpace)
	if err != nil {
		return err
	}

	aSpace.Lock()
	p, ok := aSpace.subnets[k]
	if !ok {
		aSpace.Unlock()
		return ipamapi.ErrBadPool
	}

	if address == nil {
		aSpace.Unlock()
		return ipamapi.ErrInvalidRequest
	}

	if !p.Pool.Contains(address) {
		aSpace.Unlock()
		return ipamapi.ErrIPOutOfRange
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c = aSpace.subnets[k]
	}
	aSpace.Unlock()

	mask := p.Pool.Mask

	h, err := types.GetHostPartIP(address, mask)
	if err != nil {
		return fmt.Errorf("failed to release address %s: %v", address.String(), err)
	}

	bm, err := a.retrieveBitmask(k, c.Pool)
	if err != nil {
		return fmt.Errorf("could not find bitmask in datastore for %s on address %v release from pool %s: %v",
			k.String(), address, poolID, err)
	}

	return bm.Unset(ipToUint64(h))
}

func (a *Allocator) getAddress(nw *net.IPNet, bitmask *bitseq.Handle, prefAddress net.IP, ipr *AddressRange) (net.IP, error) {
	var (
		ordinal uint64
		err     error
		base    *net.IPNet
	)

	base = types.GetIPNetCopy(nw)

	if bitmask.Unselected() <= 0 {
		return nil, ipamapi.ErrNoAvailableIPs
	}
	if ipr == nil && prefAddress == nil {
		ordinal, err = bitmask.SetAny()
	} else if prefAddress != nil {
		hostPart, e := types.GetHostPartIP(prefAddress, base.Mask)
		if e != nil {
			return nil, fmt.Errorf("failed to allocate preferred address %s: %v", prefAddress.String(), e)
		}
		ordinal = ipToUint64(types.GetMinimalIP(hostPart))
		err = bitmask.Set(ordinal)
	} else {
		ordinal, err = bitmask.SetAnyInRange(ipr.Start, ipr.End)
	}
	if err != nil {
		return nil, ipamapi.ErrNoAvailableIPs
	}

	// Convert IP ordinal for this subnet into IP address
	return generateAddress(ordinal, base), nil
}

// DumpDatabase dumps the internal info
func (a *Allocator) DumpDatabase() string {
	a.Lock()
	defer a.Unlock()

	var s string
	for as, aSpace := range a.addrSpaces {
		s = fmt.Sprintf("\n\n%s Config", as)
		aSpace.Lock()
		for k, config := range aSpace.subnets {
			s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n%v: %v", k, config))
		}
		aSpace.Unlock()
	}

	s = fmt.Sprintf("%s\n\nBitmasks", s)
	for k, bm := range a.addresses {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n\t%s: %s\n\t%d", k, bm, bm.Unselected()))
	}

	return s
}
