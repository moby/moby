// Package ipamutils provides utility functions for ipam management
package ipamutils

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/docker/docker/libnetwork/ipbits"
)

var (
	// defaultLocalScopeSubnetter contains a list of 31 IPv4 private networks with host size 16 and 12
	// (172.17-31.x.x/16, 192.168.x.x/20) which do not overlap with the networks in `defaultGlobalScopeSubnetter`
	defaultLocalScopeSubnetter Subnetter
	// defaultGlobalScopeSubnetter contains a list of 64K IPv4 private networks with host size 8
	// (10.x.x.x/24) which do not overlap with the networks in `defaultLocalScopeSubnetter`
	defaultGlobalScopeSubnetter Subnetter
	mutex                       sync.Mutex
	defaultLocalScopeNetworks   = []*NetworkToSplit{
		{Base: "172.17.0.0/16", Size: 16},
		{Base: "172.18.0.0/16", Size: 16},
		{Base: "172.19.0.0/16", Size: 16},
		{Base: "172.20.0.0/14", Size: 16},
		{Base: "172.24.0.0/14", Size: 16},
		{Base: "172.28.0.0/14", Size: 16},
		{Base: "192.168.0.0/16", Size: 20}}
	defaultGlobalScopeNetworks = []*NetworkToSplit{
		{Base: "10.0.0.0/8", Size: 24}}
)

// IPVersion represents the type of IP address.
type IPVersion int

// Constants for the IP address types.
const (
	IPv4 IPVersion = 4
	IPv6 IPVersion = 6
)

func IPVerFromPrefix(p netip.Prefix) IPVersion {
	if p.Addr().BitLen() == 32 {
		return IPv4
	}
	return IPv6
}

// AddrLen returns the length in bits of an address
func (ipVer IPVersion) AddrLen() int {
	switch {
	case ipVer == IPv4:
		return 32
	case ipVer == IPv6:
		return 128
	default:
		panic(fmt.Sprintf("IPv%d is an invalid version of IP protocol", ipVer))
	}
}

// NetworkToSplit represent a network that has to be split in chunks with mask length Size.
// Each subnet in the set is derived from the Base pool. Base is to be passed
// in CIDR format.
// Example: a Base "10.10.0.0/16 with Size 24 will define the set of 256
// 10.10.[0-255].0/24 address pools
type NetworkToSplit struct {
	Base      string
	Size      int
	ipVersion IPVersion
	// maxOffset stores the number of iterations that could be done on Base to produce subnets of the desired Size
	maxOffset uint64
	// Used to cache the transformation of Base into netip.Prefix
	prefix netip.Prefix
}

func init() {
	var err error
	if defaultGlobalScopeSubnetter, err = NewSubnetter(defaultGlobalScopeNetworks); err != nil {
		panic("failed to initialize the global scope default address pool: " + err.Error())
	}

	if defaultLocalScopeSubnetter, err = NewSubnetter(defaultLocalScopeNetworks); err != nil {
		panic("failed to initialize the local scope default address pool: " + err.Error())
	}
}

// ConfigGlobalScopeDefaultNetworks configures global default pool.
// Ideally this will be called from SwarmKit as part of swarm init
func ConfigGlobalScopeDefaultNetworks(defaultAddressPool []*NetworkToSplit) error {
	if defaultAddressPool == nil {
		return nil
	}
	mutex.Lock()
	defer mutex.Unlock()
	defaultSubnetter, err := NewSubnetter(defaultAddressPool)
	if err != nil {
		return err
	}
	defaultGlobalScopeSubnetter = defaultSubnetter
	return nil
}

// GetDefaultGlobalScopeSubnetter returns a copy of the default global-sopce Subnetter.
func GetDefaultGlobalScopeSubnetter() Subnetter {
	mutex.Lock()
	defer mutex.Unlock()
	return defaultGlobalScopeSubnetter
}

// GetDefaultLocalScopeSubnetter returns a copy of the default local-scope Subnetter.
func GetDefaultLocalScopeSubnetter() Subnetter {
	return defaultLocalScopeSubnetter
}

// Subnetter is an iterator lazily enumerating subnets from its NetworkToSplit. IPv6
// subnets can't be enumerated eagerly, otherwise they might end up taking way too much
// memory (eg. fd00::/8 split into /96). See https://github.com/moby/moby/issues/40275.
// Subnetter is not safe for concurrent use.
type Subnetter struct {
	networks []NetworkToSplit
	// index tracks what NetworkToSplit should be used for next allocation.
	index int
	// offset tracks how much of the current NetworkToSplit (indicated by index) has been consumed so far. It's
	// reset every time index changes.
	offset uint64
}

// NewSubnetter creates a Subnetter from a list of NetworkToSplit. It returns an error if one of the
// provided network is invalid (ie could not be parsed or wanted subnet Size is smaller than Base mask).
func NewSubnetter(networks []*NetworkToSplit) (Subnetter, error) {
	nets := make([]NetworkToSplit, len(networks))
	for i, network := range networks {
		nw := NetworkToSplit{Base: network.Base, Size: network.Size}
		p, err := netip.ParsePrefix(nw.Base)
		if err != nil {
			return Subnetter{}, fmt.Errorf("invalid base pool %q: %v", nw.Base, err)
		}

		ones := p.Bits()
		if nw.Size <= 0 || nw.Size < ones {
			return Subnetter{}, fmt.Errorf("invalid pools size: %d", nw.Size)
		}

		nw.prefix = p
		nw.ipVersion = IPVerFromPrefix(p)
		// When there's more than 2^64 subnets that could be enumerated from a network, just cap maxOffset
		// to 2^64 as this is already a number that couldn't be realistically reached.
		if nw.Size-ones > 64 {
			nw.maxOffset = 1<<64 - 1
		} else {
			nw.maxOffset = 1<<uint(nw.Size-ones) - 1
		}

		nets[i] = nw
	}

	return Subnetter{
		networks: nets,
	}, nil
}

// Reset resets the internal position of the iterator to the start of the collection.
func (s *Subnetter) Reset() {
	s.index = 0
	s.offset = 0
}

// ipVerSubset returns a new Subnetter containing only IPv4 or IPv6 networks depending on ipVersion parameter.
func (s *Subnetter) ipVerSubset(ipVersion IPVersion) *Subnetter {
	newS := &Subnetter{
		networks: make([]NetworkToSplit, 0),
	}

	for _, nw := range s.networks {
		if nw.ipVersion == ipVersion {
			newS.networks = append(newS.networks, nw)
		}
	}

	return newS
}

func (s *Subnetter) V4() *Subnetter {
	return s.ipVerSubset(IPv4)
}

func (s *Subnetter) V6() *Subnetter {
	return s.ipVerSubset(IPv6)
}

// EndReached indicates whether there's at least one remaining subnet from its NetworkToSplit.
func (s *Subnetter) EndReached() bool {
	if s.index >= len(s.networks) {
		return true
	}
	if len(s.networks) == 0 {
		return true
	}
	return false
}

var ErrNoMoreSubnet = errors.New("no more subnet available")

// NextSubnet returns a new subnet from its NetworkToSplit, or an error if all subnets have already been enumerated.
func (s *Subnetter) NextSubnet() (netip.Prefix, error) {
	if s.EndReached() {
		return netip.Prefix{}, ErrNoMoreSubnet
	}

	nw := s.networks[s.index]
	base := nw.prefix

	ordinal := uint(nw.ipVersion.AddrLen() - nw.Size)
	addr := ipbits.Add(base.Addr(), s.offset, ordinal)
	subnet := netip.PrefixFrom(addr, nw.Size)

	if s.offset == nw.maxOffset {
		s.index++
		s.offset = 0
	} else if s.offset < nw.maxOffset {
		s.offset++
	}

	return subnet, nil
}
