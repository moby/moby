// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.

package flate

import (
	"math/bits"

	"github.com/klauspost/compress/internal/le"
)

// matchLen returns the maximum common prefix length of a and b.
// a must be the shortest of the two.
func matchLen(a, b []byte) (n int) {
	// Keeping a and b the same length and reslicing them as we go
	// lets the compiler prove that no bounds checks are needed.
	b = b[:len(a)]
	for len(a) >= 8 {
		diff := le.Load64(a, 0) ^ le.Load64(b, 0)
		if diff != 0 {
			return n + bits.TrailingZeros64(diff)>>3
		}
		a, b = a[8:], b[8:]
		n += 8
	}
	for i := range a {
		if a[i] != b[i] {
			break
		}
		n++
	}
	return n
}
