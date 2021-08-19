// Package ipamutils provides utility functions for ipam management
package ipamutils

import (
	"fmt"
	"net"
	"sync"
)

var (
	// initLocal makes sure we initialize the defaults only once
	initLocal sync.Once
	local     sync.RWMutex

	// predefinedLocalScopeDefaultNetworks contains a list of 31 IPv4 private
	// networks with host size 16 and 12 (172.17-31.x.x/16, 192.168.x.x/20)
	// which do not overlap with the networks in predefinedGlobalScopeDefaultNetworks
	predefinedLocalScopeDefaultNetworks []*net.IPNet

	// initGlobal makes sure we initialize the defaults only once
	initGlobal sync.Once
	global     sync.RWMutex

	globalScopeDefaultNetworks = []*NetworkToSplit{{"10.0.0.0/8", 24}}

	// predefinedGlobalScopeDefaultNetworks contains a list of 64K IPv4 private
	// networks with host size 8 (10.x.x.x/24) which do not overlap with the
	// networks in predefinedLocalScopeDefaultNetworks
	predefinedGlobalScopeDefaultNetworks []*net.IPNet
)

// NetworkToSplit represent a network that has to be split in chunks with mask length Size.
// Each subnet in the set is derived from the Base pool. Base is to be passed
// in CIDR format.
// Example: a Base "10.10.0.0/16 with Size 24 will define the set of 256
// 10.10.[0-255].0/24 address pools
type NetworkToSplit struct {
	Base string `json:"base"`
	Size int    `json:"size"`
}

// initLocalScopeDefaultNetworks initializes the default address pools for the
// local scope.
func initLocalScopeDefaultNetworks() {
	var err error
	predefinedLocalScopeDefaultNetworks, err = splitNetworks([]*NetworkToSplit{
		{"172.17.0.0/16", 16},
		{"172.18.0.0/16", 16},
		{"172.19.0.0/16", 16},
		{"172.20.0.0/14", 16},
		{"172.24.0.0/14", 16},
		{"172.28.0.0/14", 16},
		{"192.168.0.0/16", 20},
	})
	if err != nil {
		panic("failed to initialize the local scope default address pool: " + err.Error())
	}
}

// initGlobalScopeDefaultNetworks initializes the default address pools
func initGlobalScopeDefaultNetworks() {
	var err error
	predefinedGlobalScopeDefaultNetworks, err = splitNetworks(globalScopeDefaultNetworks)
	if err != nil {
		panic("failed to initialize the global scope default address pool: " + err.Error())
	}
}

// GetGlobalScopeDefaultNetworks returns a list of 64K IPv4 private networks with
// host size 8 (10.x.x.x/24) which do not overlap with the networks returned by
// GetLocalScopeDefaultNetworks
func GetGlobalScopeDefaultNetworks() []*net.IPNet {
	global.RLock()
	defer global.RUnlock()
	initGlobal.Do(initGlobalScopeDefaultNetworks)

	return predefinedGlobalScopeDefaultNetworks
}

// GetLocalScopeDefaultNetworks returns a list of 31 IPv4 private networks with
// host size 16 and 12 (172.17-31.x.x/16, 192.168.x.x/20) which do not overlap
// with the networks returned by GetGlobalScopeDefaultNetworks.
func GetLocalScopeDefaultNetworks() []*net.IPNet {
	initLocal.Do(initLocalScopeDefaultNetworks)

	local.RLock()
	defer local.RUnlock()
	return predefinedLocalScopeDefaultNetworks
}

// ConfigGlobalScopeDefaultNetworks configures global default pool.
// Ideally this will be called from SwarmKit as part of swarm init
func ConfigGlobalScopeDefaultNetworks(defaultAddressPool []*NetworkToSplit) error {
	global.Lock()
	defer global.Unlock()

	if defaultAddressPool == nil {
		// reset to defaults
		defaultAddressPool = globalScopeDefaultNetworks
	}

	// Prevent potential race conditions; trigger setting the defaults if it was
	// not run yet, otherwise a call to GetLocalScopeDefaultNetworks or
	// GetLocalScopeDefaultNetworks would overwrite the custom configuration.
	initGlobal.Do(initGlobalScopeDefaultNetworks)

	nws, err := splitNetworks(defaultAddressPool)
	if err != nil {
		return err
	}
	predefinedGlobalScopeDefaultNetworks = nws
	return nil
}

// ConfigLocalScopeDefaultNetworks configures local default pool.
// Ideally this will be called during libnetwork init
func ConfigLocalScopeDefaultNetworks(defaultAddressPool []*NetworkToSplit) error {
	if defaultAddressPool == nil {
		return nil
	}

	// Prevent potential race conditions; trigger setting the defaults if it was
	// not run yet, otherwise a call to GetLocalScopeDefaultNetworks or
	// GetLocalScopeDefaultNetworks would overwrite the custom configuration.
	initLocal.Do(initLocalScopeDefaultNetworks)

	local.Lock()
	defer local.Unlock()
	nws, err := splitNetworks(defaultAddressPool)
	if err != nil {
		return err
	}
	predefinedLocalScopeDefaultNetworks = nws
	return nil
}

// splitNetworks takes a slice of networks, split them accordingly and returns them
func splitNetworks(list []*NetworkToSplit) ([]*net.IPNet, error) {
	localPools := make([]*net.IPNet, 0, len(list))

	for _, p := range list {
		_, b, err := net.ParseCIDR(p.Base)
		if err != nil {
			return nil, fmt.Errorf("invalid base pool %q: %v", p.Base, err)
		}
		ones, _ := b.Mask.Size()
		if p.Size <= 0 || p.Size < ones {
			return nil, fmt.Errorf("invalid pools size: %d", p.Size)
		}
		localPools = append(localPools, splitNetwork(p.Size, b)...)
	}
	return localPools, nil
}

func splitNetwork(size int, base *net.IPNet) []*net.IPNet {
	one, bits := base.Mask.Size()
	mask := net.CIDRMask(size, bits)
	n := 1 << uint(size-one)
	s := uint(bits - size)
	list := make([]*net.IPNet, 0, n)

	for i := 0; i < n; i++ {
		ip := make([]byte, len(base.IP))
		copy(ip, base.IP)
		addIntToIP(ip, uint(i<<s))
		list = append(list, &net.IPNet{IP: ip, Mask: mask})
	}
	return list
}

func addIntToIP(array net.IP, ordinal uint) {
	for i := len(array) - 1; i >= 0; i-- {
		array[i] |= (byte)(ordinal & 0xff)
		ordinal >>= 8
	}
}
