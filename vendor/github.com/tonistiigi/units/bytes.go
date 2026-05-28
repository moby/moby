/*
	Simple byte size formatting.

	This package implements types that can be used in stdlib formatting functions
	like `fmt.Printf` to control the output of the expected printed string.


	Floating point flags %f and %g print the value in using the correct unit
	suffix. Decimal units are default, # switches to binary units. If a value is
	best represented as full bytes, integer bytes are printed instead.

		Examples:
			fmt.Printf("%.2f", 123 * B)   => "123B"
			fmt.Printf("%.2f", 1234 * B)  => "1.23kB"
			fmt.Printf("%g", 1200 * B)    => "1.2kB"
			fmt.Printf("%#g", 1024 * B)   => "1KiB"


	Integer flag %d always prints the value in bytes. # flag adds an unit prefix.

		Examples:
			fmt.Printf("%d", 1234 * B)    => "1234"
			fmt.Printf("%#d", 1234 * B)   => "1234B"

	%v is equal to %g

*/
package units

import (
	"fmt"
	"io"
	"math"
	"math/big"
)

type Bytes int64

const (
	B Bytes = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
	PiB
	EiB

	KB = 1e3 * B
	MB = 1e3 * KB
	GB = 1e3 * MB
	TB = 1e3 * GB
	PB = 1e3 * TB
	EB = 1e3 * PB
)

var units = map[bool][]string{
	false: []string{
		"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB",
	},
	true: []string{
		"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB",
	},
}

func (b Bytes) Format(f fmt.State, c rune) {
	switch c {
	case 'f', 'g':
		fv, unit, ok := b.floatValue(f.Flag('#'))
		if !ok {
			b.formatInt(&noPrecision{f}, 'd', true)
			return
		}
		big.NewFloat(fv).Format(f, c)
		io.WriteString(f, unit)
	case 'd':
		b.formatInt(f, c, f.Flag('#'))
	default:
		if f.Flag('#') {
			fmt.Fprintf(f, "bytes(%d)", int64(b))
		} else {
			fmt.Fprintf(f, "%g", b)
		}
	}
}

func (b Bytes) formatInt(f fmt.State, c rune, withUnit bool) {
	big.NewInt(int64(b)).Format(f, c)
	if withUnit {
		io.WriteString(f, "B")
	}
}

func (b Bytes) floatValue(binary bool) (float64, string, bool) {
	i := 0
	var baseUnit Bytes = 1
	if b < 0 {
		baseUnit *= -1
	}
	for {
		next := baseUnit
		if binary {
			next *= 1 << 10
		} else {
			next *= 1e3
		}
		if (baseUnit > 0 && b >= next) || (baseUnit < 0 && b <= next) {
			i++
			baseUnit = next
			continue
		}
		if i == 0 {
			return 0, "", false
		}

		return float64(b) / math.Abs(float64(baseUnit)), units[binary][i], true
	}
}

type noPrecision struct {
	fmt.State
}

func (*noPrecision) Precision() (prec int, ok bool) {
	return 0, false
}
