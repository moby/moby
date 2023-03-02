// Package ipbits contains utilities for manipulating [netip.Addr] values as
// numbers or bitfields.
package ipbits

import (
	"encoding/binary"
	"net/netip"
)

// Add returns ip + (x << shift).
func Add(ip netip.Addr, x uint64, shift uint) netip.Addr {
	if ip.Is4() {
		a := ip.As4()
		addr := binary.BigEndian.Uint32(a[:])
		addr += uint32(x) << shift
		binary.BigEndian.PutUint32(a[:], addr)
		return netip.AddrFrom4(a)
	} else {
		a := ip.As16()
		addr := uint128From16(a)
		addr = addr.add(uint128From(x).lsh(shift))
		addr.fill16(&a)
		return netip.AddrFrom16(a)
	}
}

// Field returns the value of the bitfield [u, v] in ip as an integer,
// where bit 0 is the most-significant bit of ip.
//
// The result is undefined if u > v, if v-u > 64, or if u or v is larger than
// ip.BitLen().
func Field(ip netip.Addr, u, v uint) uint64 {
	if ip.Is4() {
		mask := ^uint32(0) >> u
		a := ip.As4()
		return uint64((binary.BigEndian.Uint32(a[:]) & mask) >> (32 - v))
	} else {
		mask := uint128From(0).not().rsh(u)
		return uint128From16(ip.As16()).and(mask).rsh(128 - v).uint64()
	}
}
