//go:build !linux && !windows

package netutils

import "net/netip"

// InferReservedNetworks returns an empty list on unsupported platforms.
func InferReservedNetworks(v6 bool) []netip.Prefix {
	return nil
}
