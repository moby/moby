package ipam

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
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
	subnets map[SubnetKey]*PoolData
	// Allocated addresses in each address space's subnet
	addresses map[SubnetKey]*bitseq.Handle
	// Datastore
	store    datastore.DataStore
	dbIndex  uint64
	dbExists bool
	sync.Mutex
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(lcDs, glDs datastore.DataStore) (*Allocator, error) {
	a := &Allocator{}
	a.subnets = make(map[SubnetKey]*PoolData)
	a.addresses = make(map[SubnetKey]*bitseq.Handle)
	a.predefined = make(map[string][]*net.IPNet, 2)
	a.predefined[localAddressSpace] = initLocalPredefinedPools()
	a.predefined[globalAddressSpace] = initGlobalPredefinedPools()
	a.store = glDs

	if a.store == nil {
		return a, nil
	}

	// Register for status changes
	a.watchForChanges()

	// Get the initial subnet configs status from the ds if present.
	kvPair, err := a.store.KVStore().Get(datastore.Key(a.Key()...))
	if err != nil {
		if err != store.ErrKeyNotFound {
			return nil, fmt.Errorf("failed to retrieve the ipam subnet configs from datastore: %v", err)
		}
		return a, nil
	}
	a.subnetConfigFromStore(kvPair)

	// Now retrieve the bitmasks for the master pools
	var inserterList []func() error
	a.Lock()
	for k, v := range a.subnets {
		if v.Range == nil {
			inserterList = append(inserterList, func() error { return a.insertBitMask(k, v.Pool) })
		}
	}
	a.Unlock()

	// Add the bitmasks, data could come from datastore
	for _, f := range inserterList {
		if err := f(); err != nil {
			return nil, err
		}
	}

	return a, nil
}

func (a *Allocator) subnetConfigFromStore(kvPair *store.KVPair) {
	a.Lock()
	if a.dbIndex < kvPair.LastIndex {
		a.SetValue(kvPair.Value)
		a.dbIndex = kvPair.LastIndex
		a.dbExists = true
	}
	a.Unlock()
}

// SubnetKey is the pointer to the configured pools in each address space
type SubnetKey struct {
	AddressSpace string
	Subnet       string
	ChildSubnet  string
}

// String returns the string form of the SubnetKey object
func (s *SubnetKey) String() string {
	k := fmt.Sprintf("%s/%s", s.AddressSpace, s.Subnet)
	if s.ChildSubnet != "" {
		k = fmt.Sprintf("%s/%s", k, s.ChildSubnet)
	}
	return k
}

// FromString populate the SubnetKey object reading it from string
func (s *SubnetKey) FromString(str string) error {
	if str == "" || !strings.Contains(str, "/") {
		return fmt.Errorf("invalid string form for subnetkey: %s", str)
	}

	p := strings.Split(str, "/")
	if len(p) != 3 && len(p) != 5 {
		return fmt.Errorf("invalid string form for subnetkey: %s", str)
	}
	s.AddressSpace = p[0]
	s.Subnet = fmt.Sprintf("%s/%s", p[1], p[2])
	if len(p) == 5 {
		s.ChildSubnet = fmt.Sprintf("%s/%s", p[3], p[4])
	}

	return nil
}

// AddressRange specifies first and last ip ordinal which
// identify a range in a a pool of addresses
type AddressRange struct {
	Sub        *net.IPNet
	Start, End uint32
}

// String returns the string form of the AddressRange object
func (r *AddressRange) String() string {
	return fmt.Sprintf("Sub: %s, range [%d, %d]", r.Sub, r.Start, r.End)
}

// MarshalJSON returns the JSON encoding of the Range object
func (r *AddressRange) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"Sub":   r.Sub.String(),
		"Start": r.Start,
		"End":   r.End,
	}
	return json.Marshal(m)
}

// UnmarshalJSON decodes data into the Range object
func (r *AddressRange) UnmarshalJSON(data []byte) error {
	m := map[string]interface{}{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	if r.Sub, err = types.ParseCIDR(m["Sub"].(string)); err != nil {
		return err
	}
	r.Start = uint32(m["Start"].(float64))
	r.End = uint32(m["End"].(float64))
	return nil
}

// PoolData contains the configured pool data
type PoolData struct {
	ParentKey SubnetKey
	Pool      *net.IPNet
	Range     *AddressRange `json:",omitempty"`
	RefCount  int
}

// String returns the string form of the PoolData object
func (p *PoolData) String() string {
	return fmt.Sprintf("ParentKey: %s, Pool: %s, Range: %s, RefCount: %d",
		p.ParentKey.String(), p.Pool.String(), p.Range, p.RefCount)
}

// MarshalJSON returns the JSON encoding of the PoolData object
func (p *PoolData) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"ParentKey": p.ParentKey,
		"RefCount":  p.RefCount,
	}
	if p.Pool != nil {
		m["Pool"] = p.Pool.String()
	}
	if p.Range != nil {
		m["Range"] = p.Range
	}
	return json.Marshal(m)
}

// UnmarshalJSON decodes data into the PoolData object
func (p *PoolData) UnmarshalJSON(data []byte) error {
	var (
		err error
		t   struct {
			ParentKey SubnetKey
			Pool      string
			Range     *AddressRange `json:",omitempty"`
			RefCount  int
		}
	)

	if err = json.Unmarshal(data, &t); err != nil {
		return err
	}

	p.ParentKey = t.ParentKey
	p.Range = t.Range
	p.RefCount = t.RefCount
	if t.Pool != "" {
		if p.Pool, err = types.ParseCIDR(t.Pool); err != nil {
			return err
		}
	}

	return nil
}

type ipVersion int

const (
	v4 = 4
	v6 = 6
)

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
retry:
	insert, err := a.updatePoolDBOnAdd(*k, nw, ipr)
	if err != nil {
		return "", nil, nil, err
	}
	if err := a.writeToStore(); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return "", nil, nil, types.InternalErrorf("pool configuration failed because of %s", err.Error())
		}
		if erru := a.readFromStore(); erru != nil {
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

retry:
	remove, err := a.updatePoolDBOnRemoval(k)
	if err != nil {
		return err
	}
	if err = a.writeToStore(); err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return types.InternalErrorf("pool (%s) removal failed because of %v", poolID, err)
		}
		if erru := a.readFromStore(); erru != nil {
			return fmt.Errorf("failed to get updated pool config from datastore (%v) after (%v)", erru, err)
		}
		goto retry
	}

	return remove()
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

func (a *Allocator) updatePoolDBOnAdd(k SubnetKey, nw *net.IPNet, ipr *AddressRange) (func() error, error) {
	a.Lock()
	defer a.Unlock()

	// Check if already allocated
	if p, ok := a.subnets[k]; ok {
		a.incRefCount(p, 1)
		return func() error { return nil }, nil
	}

	// If master pool, check for overlap
	if ipr == nil {
		if a.contains(k.AddressSpace, nw) {
			return nil, ipamapi.ErrPoolOverlap
		}
		// This is a new master pool, add it along with corresponding bitmask
		a.subnets[k] = &PoolData{Pool: nw, RefCount: 1}
		return func() error { return a.insertBitMask(k, nw) }, nil
	}

	// This is a new non-master pool
	p := &PoolData{
		ParentKey: SubnetKey{AddressSpace: k.AddressSpace, Subnet: k.Subnet},
		Pool:      nw,
		Range:     ipr,
		RefCount:  1,
	}
	a.subnets[k] = p

	// Look for parent pool
	pp, ok := a.subnets[p.ParentKey]
	if ok {
		a.incRefCount(pp, 1)
		return func() error { return nil }, nil
	}

	// Parent pool does not exist, add it along with corresponding bitmask
	a.subnets[p.ParentKey] = &PoolData{Pool: nw, RefCount: 1}
	return func() error { return a.insertBitMask(p.ParentKey, nw) }, nil
}

func (a *Allocator) updatePoolDBOnRemoval(k SubnetKey) (func() error, error) {
	a.Lock()
	defer a.Unlock()

	p, ok := a.subnets[k]
	if !ok {
		return nil, ipamapi.ErrBadPool
	}

	a.incRefCount(p, -1)

	c := p
	for ok {
		if c.RefCount == 0 {
			delete(a.subnets, k)
			if c.Range == nil {
				return func() error {
					bm, err := a.retrieveBitmask(k, c.Pool)
					if err != nil {
						return fmt.Errorf("could not find bitmask in datastore for pool %s removal: %v", k.String(), err)
					}
					return bm.Destroy()
				}, nil
			}
		}
		k = c.ParentKey
		c, ok = a.subnets[k]
	}

	return func() error { return nil }, nil
}

func (a *Allocator) incRefCount(p *PoolData, delta int) {
	c := p
	ok := true
	for ok {
		c.RefCount += delta
		c, ok = a.subnets[c.ParentKey]
	}
}

func (a *Allocator) insertBitMask(key SubnetKey, pool *net.IPNet) error {
	log.Debugf("Inserting bitmask (%s, %s)", key.String(), pool.String())
	ipVer := getAddressVersion(pool.IP)
	ones, bits := pool.Mask.Size()
	numAddresses := uint32(1 << uint(bits-ones))

	if ipVer == v4 {
		// Do not let broadcast address be reserved
		numAddresses--
	}

	// Generate the new address masks. AddressMask content may come from datastore
	h, err := bitseq.NewHandle(dsDataKey, a.store, key.String(), numAddresses)
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

	for _, nw := range a.getPredefineds(as) {
		if v != getAddressVersion(nw.IP) {
			continue
		}
		a.Lock()
		_, ok := a.subnets[SubnetKey{AddressSpace: as, Subnet: nw.String()}]
		a.Unlock()
		if ok {
			continue
		}

		if !a.contains(as, nw) {
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

// Check subnets size. In case configured subnet is v6 and host size is
// greater than 32 bits, adjust subnet to /96.
func adjustAndCheckSubnetSize(subnet *net.IPNet) (*net.IPNet, error) {
	ones, bits := subnet.Mask.Size()
	if v6 == getAddressVersion(subnet.IP) {
		if ones < minNetSizeV6 {
			return nil, ipamapi.ErrInvalidPool
		}
		if ones < minNetSizeV6Eff {
			newMask := net.CIDRMask(minNetSizeV6Eff, bits)
			return &net.IPNet{IP: subnet.IP, Mask: newMask}, nil
		}
	} else {
		if ones < minNetSize {
			return nil, ipamapi.ErrInvalidPool
		}
	}
	return subnet, nil
}

// Checks whether the passed subnet is a superset or subset of any of the subset in the db
func (a *Allocator) contains(space string, nw *net.IPNet) bool {
	for k, v := range a.subnets {
		if space == k.AddressSpace && k.ChildSubnet == "" {
			if nw.Contains(v.Pool.IP) || v.Pool.Contains(nw.IP) {
				return true
			}
		}
	}
	return false
}

// RequestAddress returns an address from the specified pool ID
func (a *Allocator) RequestAddress(poolID string, prefAddress net.IP, opts map[string]string) (*net.IPNet, map[string]string, error) {
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return nil, nil, types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	a.Lock()
	p, ok := a.subnets[k]
	if !ok {
		a.Unlock()
		return nil, nil, types.NotFoundErrorf("cannot find address pool for poolID:%s", poolID)
	}

	if prefAddress != nil && !p.Pool.Contains(prefAddress) {
		a.Unlock()
		return nil, nil, ipamapi.ErrIPOutOfRange
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c, ok = a.subnets[k]
	}
	a.Unlock()

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
	k := SubnetKey{}
	if err := k.FromString(poolID); err != nil {
		return types.BadRequestErrorf("invalid pool id: %s", poolID)
	}

	a.Lock()
	p, ok := a.subnets[k]
	if !ok {
		a.Unlock()
		return ipamapi.ErrBadPool
	}

	if address == nil || !p.Pool.Contains(address) {
		a.Unlock()
		return ipamapi.ErrInvalidRequest
	}

	c := p
	for c.Range != nil {
		k = c.ParentKey
		c = a.subnets[k]
	}
	a.Unlock()

	mask := p.Pool.Mask
	if p.Range != nil {
		mask = p.Range.Sub.Mask
	}
	h, err := types.GetHostPartIP(address, mask)
	if err != nil {
		return fmt.Errorf("failed to release address %s: %v", address.String(), err)
	}

	bm, err := a.retrieveBitmask(k, c.Pool)
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
	} else if ipr != nil {
		base.IP = ipr.Sub.IP
		ordinal, err = bitmask.SetAnyInRange(ipr.Start, ipr.End)
	} else {
		hostPart, e := types.GetHostPartIP(prefAddress, base.Mask)
		if e != nil {
			return nil, fmt.Errorf("failed to allocate preferred address %s: %v", prefAddress.String(), e)
		}
		ordinal = ipToUint32(types.GetMinimalIP(hostPart))
		err = bitmask.Set(ordinal)
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

	s := fmt.Sprintf("\n\nPoolData")
	for k, config := range a.subnets {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n%v: %v", k, config))
	}

	s = fmt.Sprintf("%s\n\nBitmasks", s)
	for k, bm := range a.addresses {
		s = fmt.Sprintf("%s%s", s, fmt.Sprintf("\n\t%s: %s\n\t%d", k, bm, bm.Unselected()))
	}
	return s
}

// It generates the ip address in the passed subnet specified by
// the passed host address ordinal
func generateAddress(ordinal uint32, network *net.IPNet) net.IP {
	var address [16]byte

	// Get network portion of IP
	if getAddressVersion(network.IP) == v4 {
		copy(address[:], network.IP.To4())
	} else {
		copy(address[:], network.IP)
	}

	end := len(network.Mask)
	addIntToIP(address[:end], ordinal)

	return net.IP(address[:end])
}

func getAddressVersion(ip net.IP) ipVersion {
	if ip.To4() == nil {
		return v6
	}
	return v4
}

// Adds the ordinal IP to the current array
// 192.168.0.0 + 53 => 192.168.53
func addIntToIP(array []byte, ordinal uint32) {
	for i := len(array) - 1; i >= 0; i-- {
		array[i] |= (byte)(ordinal & 0xff)
		ordinal >>= 8
	}
}

// Convert an ordinal to the respective IP address
func ipToUint32(ip []byte) uint32 {
	value := uint32(0)
	for i := 0; i < len(ip); i++ {
		j := len(ip) - 1 - i
		value += uint32(ip[i]) << uint(j*8)
	}
	return value
}

func initLocalPredefinedPools() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 274)
	mask := []byte{255, 255, 0, 0}
	for i := 17; i < 32; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, byte(i), 0, 0}, Mask: mask})
	}
	for i := 0; i < 256; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), 0, 0}, Mask: mask})
	}
	mask24 := []byte{255, 255, 255, 0}
	for i := 42; i < 45; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{192, 168, byte(i), 0}, Mask: mask24})
	}
	return pl
}

func initGlobalPredefinedPools() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 256*256)
	mask := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), byte(j), 0}, Mask: mask})
		}
	}
	return pl
}

func getAddressRange(pool string) (*AddressRange, error) {
	ip, nw, err := net.ParseCIDR(pool)
	if err != nil {
		return nil, ipamapi.ErrInvalidSubPool
	}
	lIP, e := types.GetHostPartIP(nw.IP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's lowest ip address: %v", e)
	}
	bIP, e := types.GetBroadcastIP(nw.IP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's broadcast ip address: %v", e)
	}
	hIP, e := types.GetHostPartIP(bIP, nw.Mask)
	if e != nil {
		return nil, fmt.Errorf("failed to compute range's highest ip address: %v", e)
	}
	nw.IP = ip
	return &AddressRange{nw, ipToUint32(types.GetMinimalIP(lIP)), ipToUint32(types.GetMinimalIP(hIP))}, nil
}
