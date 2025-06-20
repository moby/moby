// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

// Package ipamutils provides utility functions for ipam management
package ipamutils

import (
	"net/netip"
	"slices"
)

var (
	localScopeDefaultNetworks = []*NetworkToSplit{
		{netip.MustParsePrefix("172.17.0.0/16"), 16},
		{netip.MustParsePrefix("172.18.0.0/16"), 16},
		{netip.MustParsePrefix("172.19.0.0/16"), 16},
		{netip.MustParsePrefix("172.20.0.0/14"), 16},
		{netip.MustParsePrefix("172.24.0.0/14"), 16},
		{netip.MustParsePrefix("172.28.0.0/14"), 16},
		{netip.MustParsePrefix("192.168.0.0/16"), 20},
	}
	globalScopeDefaultNetworks = []*NetworkToSplit{
		{netip.MustParsePrefix("10.0.0.0/8"), 24},
	}
)

// NetworkToSplit represent a network that has to be split in chunks with mask length Size.
// Each subnet in the set is derived from the Base pool. Base is to be passed
// in CIDR format.
// Example: a Base "10.10.0.0/16 with Size 24 will define the set of 256
// 10.10.[0-255].0/24 address pools
type NetworkToSplit struct {
	Base netip.Prefix `json:"base"`
	Size int          `json:"size"`
}

// FirstPrefix returns the first prefix available in NetworkToSplit.
func (n NetworkToSplit) FirstPrefix() netip.Prefix {
	return netip.PrefixFrom(n.Base.Addr(), n.Size)
}

// Overlaps is a util function checking whether 'p' overlaps with 'n'.
func (n NetworkToSplit) Overlaps(p netip.Prefix) bool {
	return n.Base.Overlaps(p)
}

// CompareWithPreferredSize is used to order default networks if a preferred size is requested.
// The goal is to ensure that we attempt to allocate a network that is closest to the preferred
// size while not exceeding the default size of a given network.
func (n *NetworkToSplit) CompareWithPreferredSize(other *NetworkToSplit, preferredSize int) int {
	// If there's no preferred size, both networks are equivalent.
	if preferredSize <= 0 {
		return 0
	}

	contains := preferredSize >= n.Size
	isValid := netip.PrefixFrom(n.Base.Addr(), preferredSize).IsValid()
	otherContains := preferredSize >= other.Size
	otherIsValid := netip.PrefixFrom(other.Base.Addr(), preferredSize).IsValid()

	if isValid && !otherIsValid {
		// The preferred size is an invalid prefix size for 'pdf' B, so prefer A.
		return -1
	} else if otherIsValid && !isValid {
		// The preferred size is an invalid prefix size for 'pdf' A, so prefer B.
		return 1
	} else if !isValid && !otherIsValid {
		// Preferred size isn't a valid prefix size for either 'pdf'.
		// Prefer the 'pdf' with the smaller default prefix size.
		return other.Size - n.Size
	}

	// The preferred size is a valid prefix for both A and B, so now we want to sort the
	// 'pdf' networks by how closely they match the preferred size.
	if contains && !otherContains {
		// The default size for A is able to accommodate the preferred size, but B cannot.
		// So we prefer A.
		return -1
	} else if otherContains && !contains {
		// The default size for B is able to accommodate the preferred size, but A cannot.
		// So we prefer B.
		return 1
	} else if !contains && !otherContains {
		// If the event that the default size for both 'pdf' cannot contain the preferred size,
		// we prfer the 'pdf' with the larger default size.
		return n.Size - other.Size
	} else {
		// If both 'pdf' can accommodate the preferred prefix size, we prefer the one with the
		// smaller default prefix size.
		return other.Size - n.Size
	}
}

// GetGlobalScopeDefaultNetworks returns a copy of the global-scope network list.
func GetGlobalScopeDefaultNetworks() []*NetworkToSplit {
	return slices.Clone(globalScopeDefaultNetworks)
}

// GetLocalScopeDefaultNetworks returns a copy of the default local-scope network list.
func GetLocalScopeDefaultNetworks() []*NetworkToSplit {
	return slices.Clone(localScopeDefaultNetworks)
}
