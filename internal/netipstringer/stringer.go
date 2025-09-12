// Package netipstringer provides utilities to convert netip types to strings
// which return the empty string for invalid values.
package netipstringer

import (
	"net/netip"
)

// Addr returns the string representation of addr.
// The empty string is returned if addr is not valid.
func Addr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

// Prefix returns the string representation of prefix.
// The empty string is returned if prefix is not valid.
func Prefix(prefix netip.Prefix) string {
	if !prefix.IsValid() {
		return ""
	}
	return prefix.String()
}
