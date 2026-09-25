package internal

// Align returns 'n' updated to 'alignment' boundary.
func Align[I Integer](n, alignment I) I {
	return (n + alignment - 1) / alignment * alignment
}

// IsPow returns true if n is a power of two.
func IsPow[I Integer](n I) bool {
	return n != 0 && (n&(n-1)) == 0
}

// Between returns the value clamped between a and b.
func Between[I Integer](val, a, b I) I {
	lower, upper := a, b
	if lower > upper {
		upper, lower = a, b
	}

	val = min(val, upper)
	return max(val, lower)
}

// Integer represents all possible integer types.
// Remove when x/exp/constraints is moved to the standard library.
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// List of integer types known by the Go compiler. Used by TestIntegerConstraint
// to warn if a new integer type is introduced. Remove when x/exp/constraints
// is moved to the standard library.
var integers = []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr"}
