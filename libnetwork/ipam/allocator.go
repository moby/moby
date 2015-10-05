package ipam

import (
	"fmt"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/types"
)

const (
	localAddressSpace  = "LocalDefault"
	globalAddressSpace = "GlobalDefault"
	// The biggest configurable host subnets
	minNetSize      = 8
	minNetSizeV6    = 64
	minNetSizeV6Eff = 96
	// datastore keyes for ipam objects
	dsConfigKey = "ipam/" + ipamapi.DefaultIPAM + "/config"
	dsDataKey   = "ipam/" + ipamapi.DefaultIPAM + "/data"
)

// Allocator provides per address space ipv4/ipv6 book keeping
type Allocator struct {
	// Predefined pools for default address spaces
	predefined map[string][]*net.IPNet
	// Static subnet information
	localSubnets  *PoolsConfig
	globalSubnets *PoolsConfig
	// Allocated addresses in each address space's subnet
	addresses map[SubnetKey]*bitseq.Handle
	// Datastore
	addrSpace2Configs map[string]*PoolsConfig
	sync.Mutex
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(lcDs, glDs datastore.DataStore) (*Allocator, error) {
	a := &Allocator{}

	a.localSubnets = &PoolsConfig{
		subnets: map[SubnetKey]*PoolData{},
		id:      dsConfigKey + "/Pools",
		scope:   datastore.LocalScope,
		ds:      lcDs,
		alloc:   a,
	}

	a.globalSubnets = &PoolsConfig{
		subnets: map[SubnetKey]*PoolData{},
		id:      dsConfigKey + "/Pools",
		scope:   datastore.GlobalScope,
		ds:      glDs,
		alloc:   a,
	}

	a.predefined = map[string][]*net.IPNet{
		localAddressSpace:  initLocalPredefinedPools(),
		globalAddressSpace: initGlobalPredefinedPools(),
	}

	a.addrSpace2Configs = map[string]*PoolsConfig{
		localAddressSpace:  a.localSubnets,
		globalAddressSpace: a.globalSubnets,
	}

	a.addresses = make(map[SubnetKey]*bitseq.Handle)

	cfgs := []struct {
		cfg *PoolsConfig
		dsc string
	}{
		{a.localSubnets, "local"},
		{a.globalSubnets, "global"},
	}
	// Get the initial local/global pools configfrom the datastores
	var inserterList []func() error
	for _, e := range cfgs {
		if e.cfg.ds == nil {
			continue
		}
		if err := e.cfg.watchForChanges(); err != nil {
			log.Warnf("Error on registering watch for %s datastore: %v", e.dsc, err)
		}
		if err := e.cfg.readFromStore(); err != nil && err != store.ErrKeyNotFound {
			return nil, fmt.Errorf("failed to retrieve the ipam %s pools config from datastore: %v", e.dsc, err)
		}
		e.cfg.Lock()
		for k, v := range e.cfg.subnets {
			if v.Range == nil {
				inserterList = append(inserterList, func() error { return a.insertBitMask(e.cfg.ds, k, v.Pool) })
			}
		}
		e.cfg.Unlock()
	}
	// Add the bitmasks (data could come from datastore)
	if inserterList != nil {
		for _, f := range inserterList {
			if err := f(); err != nil {
				return nil, err
			}
		}
	}

	return a, nil
}

// GetDefaultAddressSpaces returns the local and global default address spaces
func (a *Allocator) GetDefaultAddressSpaces() (string, string, error) {
	return localAddressSpace, globalAddressSpace, nil
}

// RequestPool returns an address pool along with its unique id.
func (a *Allocator) RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error) {
	k, nw, aw, ipr, err := a.parsePoolRequest(addressSpace, pool, subPool, v6)
	if err != nil {
		return "", nil, nil, ipamapi.ErrInvalidPool
	}

	cfg, err := a.getPoolsConfig(addressSpace)
	if err != nil {
		return "", nil, nil, err
	}

retry:
	insert, err := cfg.updatePoolDBOnAdd(*k, nw, ipr)
	if err != nil {
		return "", nil, nil, err
	}
	if err := cfg.writeToStore(); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return "", nil, nil, types.InternalErrorf("pool configuration failed because of %s", err.Error())
		}
		if erru := cfg.readFromStore(); erru != nil {
			return "", nil, nil, fmt.Errorf("failed to get updated pool config from datastore (%v) after (%v)", erru, err)
		}
		goto retry
	}
	return k.String(), aw, nil, insert()
}

// ReleasePool releases the address pool identified by the passed id
func (a *Allocator) ReleasePool(poolID string) error {
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	cfg, err := a.getPoolsConfig(k.AddressSpace)
	if err != nil {
		return err
	}

retry:
	remove, err := cfg.updatePoolDBOnRemoval(k)
	if err != nil {
		return err
	}
	if err = cfg.writeToStore(); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return types.InternalErrorf("pool (%s) removal failed because of %v", poolID, err)
		}
		if erru := cfg.readFromStore(); erru != nil {
			return fmt.Errorf("failed to get updated pool config from datastore (%v) after (%v)", erru, err)
		}
		goto retry
	}

	return remove()
}

// Given the address space, returns the local or global PoolConfig based on the
// address space is local or global. AddressSpace locality is being registered with IPAM out of band.
func (a *Allocator) getPoolsConfig(addrSpace string) (*PoolsConfig, error) {
	a.Lock()
	defer a.Unlock()
	cfg, ok := a.addrSpace2Configs[addrSpace]
	if !ok {
		return nil, types.BadRequestErrorf("cannot find locality of address space: %s", addrSpace)
	}
	return cfg, nil
}

func (a *Allocator) parsePoolRequest(addressSpace, pool, subPool string, v6 bool) (*SubnetKey, *net.IPNet, *net.IPNet, *AddressRange, error) {
	var (
		nw, aw *net.IPNet
		ipr    *AddressRange
		err    error
	)

	if addressSpace == "" {
		return nil, nil, nil, nil, ipamapi.ErrInvalidAddressSpace
	}

	if pool == "" && subPool != "" {
		return nil, nil, nil, nil, ipamapi.ErrInvalidSubPool
	}

	if pool != "" {
		if _, nw, err = net.ParseCIDR(pool); err != nil {
			return nil, nil, nil, nil, ipamapi.ErrInvalidPool
		}
		if subPool != "" {
			if ipr, err = getAddressRange(subPool); err != nil {
				return nil, nil, nil, nil, err
			}
		}
	} else {
		if nw, err = a.getPredefinedPool(addressSpace, v6); err != nil {
			return nil, nil, nil, nil, err
		}

	}
	if aw, err = adjustAndCheckSubnetSize(nw); err != nil {
		return nil, nil, nil, nil, err
	}

	return &SubnetKey{AddressSpace: addressSpace, Subnet: nw.String(), ChildSubnet: subPool}, nw, aw, ipr, nil
}

func (a *Allocator) insertBitMask(store datastore.DataStore, key SubnetKey, pool *net.IPNet) error {
	log.Debugf("Inserting bitmask (%s, %s)", key.String(), pool.String())
	ipVer := getAddressVersion(pool.IP)
	ones, bits := pool.Mask.Size()
	numAddresses := uint32(1 << uint(bits-ones))

	if ipVer == v4 {
		// Do not let broadcast address be reserved
		numAddresses--
	}

	// Generate the new address masks. AddressMask content may come from datastore
	h, err := bitseq.NewHandle(dsDataKey, store, key.String(), numAddresses)
	if err != nil {
		return err
	}

	if ipVer == v4 {
		// Do not let network identifier address be reserved
		h.Set(0)
	}

	a.Lock()
	a.addresses[key] = h
	a.Unlock()
	return nil
}

func (a *Allocator) retrieveBitmask(ds datastore.DataStore, k SubnetKey, n *net.IPNet) (*bitseq.Handle, error) {
	a.Lock()
	bm, ok := a.addresses[k]
	a.Unlock()
	if !ok {
		log.Debugf("Retrieving bitmask (%s, %s)", k.String(), n.String())
		if err := a.insertBitMask(ds, k, n); err != nil {
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

	cfg, err := a.getPoolsConfig(as)
	if err != nil {
		return nil, err
	}

	for _, nw := range a.getPredefineds(as) {
		if v != getAddressVersion(nw.IP) {
			continue
		}
		cfg.Lock()
		_, ok := cfg.subnets[SubnetKey{AddressSpace: as, Subnet: nw.String()}]
		cfg.Unlock()
		if ok {
			continue
		}

		if !cfg.contains(as, nw) {
			if as == localAddressSpace {
				if err := netutils.CheckRouteOverlaps(nw); err == nil {
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
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return nil, nil, types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	cfg, err := a.getPoolsConfig(k.AddressSpace)
	if err != nil {
		return nil, nil, err
	}

	cfg.Lock()
	p, ok := cfg.subnets[k]
	if !ok {
		cfg.Unlock()
		return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%s", poolID)
	}

	if prefAddress != nil && !p.Pool.Contains(prefAddress) {
		cfg.Unlock()
		return nil, nil, ipamapi.ErrIPOutOfRange
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c, ok = cfg.subnets[k]
	}
	cfg.Unlock()

	bm, err := a.retrieveBitmask(cfg.ds, k, c.Pool)
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
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	cfg, err := a.getPoolsConfig(k.AddressSpace)
	if err != nil {
		return err
	}

	cfg.Lock()
	p, ok := cfg.subnets[k]
	if !ok {
		cfg.Unlock()
		return ipamapi.ErrBadPool
	}

	if address == nil || !p.Pool.Contains(address) {
		cfg.Unlock()
		return ipamapi.ErrInvalidRequest
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c = cfg.subnets[k]
	}
	cfg.Unlock()

	mask := p.Pool.Mask
	if p.Range != nil {
		mask = p.Range.Sub.Mask
	}
	h, err := types.GetHostPartIP(address, mask)
	if err != nil {
		return fmt.Errorf("failed to release address %s: %v", address.String(), err)
	}

	bm, err := cfg.alloc.retrieveBitmask(cfg.ds, k, c.Pool)
	if err != nil {
		return fmt.Errorf("could not find bitmask in datastore for %s on address %v release from pool %s: %v",
			k.String(), address, poolID, err)
	}
	return bm.Unset(ipToUint32(h))
}

func (a *Allocator) getAddress(nw *net.IPNet, bitmask *bitseq.Handle, prefAddress net.IP, ipr *AddressRange) (net.IP, error) {
	var (
		ordinal uint32
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
		ordinal = ipToUint32(types.GetMinimalIP(hostPart))
		err = bitmask.Set(ordinal)
	} else {
		base.IP = ipr.Sub.IP
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

	s := fmt.Sprintf("\n\nLocal Pool Config")
	a.localSubnets.Lock()
	for k, config := range a.localSubnets.subnets {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n%v: %v", k, config))
	}
	a.localSubnets.Unlock()

	s = fmt.Sprintf("%s\n\nGlobal Pool Config", s)
	a.globalSubnets.Lock()
	for k, config := range a.globalSubnets.subnets {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n%v: %v", k, config))
	}
	a.globalSubnets.Unlock()

	s = fmt.Sprintf("%s\n\nBitmasks", s)
	for k, bm := range a.addresses {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n\t%s: %s\n\t%d", k, bm, bm.Unselected()))
	}
	return s
}
