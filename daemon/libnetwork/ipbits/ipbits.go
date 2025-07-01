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

// SubnetsBetween computes the number of subnets of size 'sz' available between 'a1'
// and 'a2'. The result is capped at [math.MaxUint64]. It returns 0 when one of
// 'a1' or 'a2' is invalid, if both aren't of the same family, or when 'a2' is
// less than 'a1'.
func SubnetsBetween(a1 netip.Addr, a2 netip.Addr, sz int) uint64 {
	if !a1.IsValid() || !a2.IsValid() || a1.Is4() != a2.Is4() || a2.Less(a1) {
		return 0
	}

	p1, _ := a1.Prefix(sz)
	p2, _ := a2.Prefix(sz)

	return subAddr(p2.Addr(), p1.Addr()).rsh(uint(a1.BitLen() - sz)).uint64()
}

// subAddr returns 'ip1 - ip2'. Both netip.Addr have to be of the same address
// family. 'ip1' as to be greater than or equal to 'ip2'.
func subAddr(ip1 netip.Addr, ip2 netip.Addr) uint128 {
	return uint128From16(ip1.As16()).sub(uint128From16(ip2.As16()))
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
