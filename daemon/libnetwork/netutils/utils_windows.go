package netutils

import "net/netip"

// InferReservedNetworks returns an empty list on Windows.
func InferReservedNetworks(v6 bool) []netip.Prefix {
	return []netip.Prefix{}
}
