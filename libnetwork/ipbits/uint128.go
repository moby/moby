package ipbits

import (
	"encoding/binary"
	"math/bits"
)

type uint128 struct{ hi, lo uint64 }

func uint128From16(b [16]byte) uint128 {
	return uint128{
		hi: binary.BigEndian.Uint64(b[:8]),
		lo: binary.BigEndian.Uint64(b[8:]),
	}
}

func uint128From(x uint64) uint128 {
	return uint128{lo: x}
}

func (x uint128) add(y uint128) uint128 {
	lo, carry := bits.Add64(x.lo, y.lo, 0)
	hi, _ := bits.Add64(x.hi, y.hi, carry)
	return uint128{hi: hi, lo: lo}
}

func (x uint128) lsh(n uint) uint128 {
	if n > 64 {
		return uint128{hi: x.lo << (n - 64)}
	}
	return uint128{
		hi: x.hi<<n | x.lo>>(64-n),
		lo: x.lo << n,
	}
}

func (x uint128) rsh(n uint) uint128 {
	if n > 64 {
		return uint128{lo: x.hi >> (n - 64)}
	}
	return uint128{
		hi: x.hi >> n,
		lo: x.lo>>n | x.hi<<(64-n),
	}
}

func (x uint128) and(y uint128) uint128 {
	return uint128{hi: x.hi & y.hi, lo: x.lo & y.lo}
}

func (x uint128) not() uint128 {
	return uint128{hi: ^x.hi, lo: ^x.lo}
}

func (x uint128) fill16(a *[16]byte) {
	binary.BigEndian.PutUint64(a[:8], x.hi)
	binary.BigEndian.PutUint64(a[8:], x.lo)
}

func (x uint128) uint64() uint64 {
	return x.lo
}
