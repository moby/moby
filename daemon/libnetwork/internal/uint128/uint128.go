package uint128

import (
	"encoding/binary"
	"math/bits"
)

type Uint128 struct{ hi, lo uint64 }

func From16(b [16]byte) Uint128 {
	return Uint128{
		hi: binary.BigEndian.Uint64(b[:8]),
		lo: binary.BigEndian.Uint64(b[8:]),
	}
}

func From(x, y uint64) Uint128 {
	return Uint128{hi: x, lo: y}
}

func (x Uint128) Add(y Uint128) Uint128 {
	lo, carry := bits.Add64(x.lo, y.lo, 0)
	hi, _ := bits.Add64(x.hi, y.hi, carry)
	return Uint128{hi: hi, lo: lo}
}

func (x Uint128) Sub(y Uint128) Uint128 {
	lo, carry := bits.Sub64(x.lo, y.lo, 0)
	hi, _ := bits.Sub64(x.hi, y.hi, carry)
	return Uint128{hi: hi, lo: lo}
}

func (x Uint128) Lsh(n uint) Uint128 {
	if n > 64 {
		return Uint128{hi: x.lo << (n - 64)}
	}
	return Uint128{
		hi: x.hi<<n | x.lo>>(64-n),
		lo: x.lo << n,
	}
}

func (x Uint128) Rsh(n uint) Uint128 {
	if n > 64 {
		return Uint128{lo: x.hi >> (n - 64)}
	}
	return Uint128{
		hi: x.hi >> n,
		lo: x.lo>>n | x.hi<<(64-n),
	}
}

func (x Uint128) And(y Uint128) Uint128 {
	return Uint128{hi: x.hi & y.hi, lo: x.lo & y.lo}
}

func (x Uint128) Not() Uint128 {
	return Uint128{hi: ^x.hi, lo: ^x.lo}
}

func (x Uint128) Fill16(a *[16]byte) {
	binary.BigEndian.PutUint64(a[:8], x.hi)
	binary.BigEndian.PutUint64(a[8:], x.lo)
}

func (x Uint128) Uint64() uint64 {
	return x.lo
}

func (x Uint128) Uint64Sat() uint64 {
	if x.hi != 0 {
		return ^uint64(0)
	}
	return x.lo
}
