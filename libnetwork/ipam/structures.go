package ipam

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/types"
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

// PoolsConfig contains the pool configurations
type PoolsConfig struct {
	subnets  map[SubnetKey]*PoolData
	dbIndex  uint64
	dbExists bool
	id       string
	scope    datastore.DataScope
	ds       datastore.DataStore
	alloc    *Allocator
	sync.Mutex
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

// MarshalJSON returns the JSON encoding of the PoolsConfig object
func (cfg *PoolsConfig) MarshalJSON() ([]byte, error) {
	cfg.Lock()
	defer cfg.Unlock()

	m := map[string]interface{}{
		"Scope": string(cfg.scope),
	}

	if cfg.subnets != nil {
		s := map[string]*PoolData{}
		for k, v := range cfg.subnets {
			s[k.String()] = v
		}
		m["Subnets"] = s
	}

	return json.Marshal(m)
}

// UnmarshalJSON decodes data into the PoolsConfig object
func (cfg *PoolsConfig) UnmarshalJSON(data []byte) error {
	cfg.Lock()
	defer cfg.Unlock()

	m := map[string]interface{}{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}

	cfg.scope = datastore.LocalScope
	s := m["Scope"].(string)
	if s == string(datastore.GlobalScope) {
		cfg.scope = datastore.GlobalScope
	}

	if v, ok := m["Subnets"]; ok {
		sb, _ := json.Marshal(v)
		var s map[string]*PoolData
		err := json.Unmarshal(sb, &s)
		if err != nil {
			return err
		}
		for ks, v := range s {
			k := SubnetKey{}
			k.FromString(ks)
			cfg.subnets[k] = v
		}
	}

	return nil
}

func (cfg *PoolsConfig) updatePoolDBOnAdd(k SubnetKey, nw *net.IPNet, ipr *AddressRange) (func() error, error) {
	cfg.Lock()
	defer cfg.Unlock()

	// Check if already allocated
	if p, ok := cfg.subnets[k]; ok {
		cfg.incRefCount(p, 1)
		return func() error { return nil }, nil
	}

	// If master pool, check for overlap
	if ipr == nil {
		if cfg.contains(k.AddressSpace, nw) {
			return nil, ipamapi.ErrPoolOverlap
		}
		// This is a new master pool, add it along with corresponding bitmask
		cfg.subnets[k] = &PoolData{Pool: nw, RefCount: 1}
		return func() error { return cfg.alloc.insertBitMask(cfg.ds, k, nw) }, nil
	}

	// This is a new non-master pool
	p := &PoolData{
		ParentKey: SubnetKey{AddressSpace: k.AddressSpace, Subnet: k.Subnet},
		Pool:      nw,
		Range:     ipr,
		RefCount:  1,
	}
	cfg.subnets[k] = p

	// Look for parent pool
	pp, ok := cfg.subnets[p.ParentKey]
	if ok {
		cfg.incRefCount(pp, 1)
		return func() error { return nil }, nil
	}

	// Parent pool does not exist, add it along with corresponding bitmask
	cfg.subnets[p.ParentKey] = &PoolData{Pool: nw, RefCount: 1}
	return func() error { return cfg.alloc.insertBitMask(cfg.ds, p.ParentKey, nw) }, nil
}

func (cfg *PoolsConfig) updatePoolDBOnRemoval(k SubnetKey) (func() error, error) {
	cfg.Lock()
	defer cfg.Unlock()

	p, ok := cfg.subnets[k]
	if !ok {
		return nil, ipamapi.ErrBadPool
	}

	cfg.incRefCount(p, -1)

	c := p
	for ok {
		if c.RefCount == 0 {
			delete(cfg.subnets, k)
			if c.Range == nil {
				return func() error {
					bm, err := cfg.alloc.retrieveBitmask(cfg.ds, k, c.Pool)
					if err != nil {
						return fmt.Errorf("could not find bitmask in datastore for pool %s removal: %v", k.String(), err)
					}
					return bm.Destroy()
				}, nil
			}
		}
		k = c.ParentKey
		c, ok = cfg.subnets[k]
	}

	return func() error { return nil }, nil
}

func (cfg *PoolsConfig) incRefCount(p *PoolData, delta int) {
	c := p
	ok := true
	for ok {
		c.RefCount += delta
		c, ok = cfg.subnets[c.ParentKey]
	}
}

// Checks whether the passed subnet is a superset or subset of any of the subset in this config db
func (cfg *PoolsConfig) contains(space string, nw *net.IPNet) bool {
	for k, v := range cfg.subnets {
		if space == k.AddressSpace && k.ChildSubnet == "" {
			if nw.Contains(v.Pool.IP) || v.Pool.Contains(nw.IP) {
				return true
			}
		}
	}
	return false
}
