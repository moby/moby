package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

type cond byte

const (
	// condO represents (overflow) condition.
	condO cond = iota
	// condNO represents (no overflow) condition.
	condNO
	// condB represents (< unsigned) condition.
	condB
	// condNB represents (>= unsigned) condition.
	condNB
	// condZ represents (zero) condition.
	condZ
	// condNZ represents (not-zero) condition.
	condNZ
	// condBE represents (<= unsigned) condition.
	condBE
	// condNBE represents (> unsigned) condition.
	condNBE
	// condS represents (negative) condition.
	condS
	// condNS represents (not-negative) condition.
	condNS
	// condP represents (parity) condition.
	condP
	// condNP represents (not parity) condition.
	condNP
	// condL represents (< signed) condition.
	condL
	// condNL represents (>= signed) condition.
	condNL
	// condLE represents (<= signed) condition.
	condLE
	// condNLE represents (> signed) condition.
	condNLE

	condInvalid
)

func (c cond) String() string {
	switch c {
	case condO:
		return "o"
	case condNO:
		return "no"
	case condB:
		return "b"
	case condNB:
		return "nb"
	case condZ:
		return "z"
	case condNZ:
		return "nz"
	case condBE:
		return "be"
	case condNBE:
		return "nbe"
	case condS:
		return "s"
	case condNS:
		return "ns"
	case condL:
		return "l"
	case condNL:
		return "nl"
	case condLE:
		return "le"
	case condNLE:
		return "nle"
	case condP:
		return "p"
	case condNP:
		return "np"
	default:
		panic("unreachable")
	}
}

func condFromSSAIntCmpCond(origin ssa.IntegerCmpCond) cond {
	switch origin {
	case ssa.IntegerCmpCondEqual:
		return condZ
	case ssa.IntegerCmpCondNotEqual:
		return condNZ
	case ssa.IntegerCmpCondSignedLessThan:
		return condL
	case ssa.IntegerCmpCondSignedGreaterThanOrEqual:
		return condNL
	case ssa.IntegerCmpCondSignedGreaterThan:
		return condNLE
	case ssa.IntegerCmpCondSignedLessThanOrEqual:
		return condLE
	case ssa.IntegerCmpCondUnsignedLessThan:
		return condB
	case ssa.IntegerCmpCondUnsignedGreaterThanOrEqual:
		return condNB
	case ssa.IntegerCmpCondUnsignedGreaterThan:
		return condNBE
	case ssa.IntegerCmpCondUnsignedLessThanOrEqual:
		return condBE
	default:
		panic("unreachable")
	}
}

func condFromSSAFloatCmpCond(origin ssa.FloatCmpCond) cond {
	switch origin {
	case ssa.FloatCmpCondGreaterThanOrEqual:
		return condNB
	case ssa.FloatCmpCondGreaterThan:
		return condNBE
	case ssa.FloatCmpCondEqual, ssa.FloatCmpCondNotEqual, ssa.FloatCmpCondLessThan, ssa.FloatCmpCondLessThanOrEqual:
		panic(fmt.Sprintf("cond %s must be treated as a special case", origin))
	default:
		panic("unreachable")
	}
}

func (c cond) encoding() byte {
	return byte(c)
}

func (c cond) invert() cond {
	switch c {
	case condO:
		return condNO
	case condNO:
		return condO
	case condB:
		return condNB
	case condNB:
		return condB
	case condZ:
		return condNZ
	case condNZ:
		return condZ
	case condBE:
		return condNBE
	case condNBE:
		return condBE
	case condS:
		return condNS
	case condNS:
		return condS
	case condP:
		return condNP
	case condNP:
		return condP
	case condL:
		return condNL
	case condNL:
		return condL
	case condLE:
		return condNLE
	case condNLE:
		return condLE
	default:
		panic("unreachable")
	}
}
