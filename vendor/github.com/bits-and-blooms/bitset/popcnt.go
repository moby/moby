package bitset

import "math/bits"

func popcntSlice(s []uint64) (cnt uint64) {
	// Unroll the loop with four independent accumulators to break the
	// dependency chain on cnt and let the CPU run multiple popcounts in
	// parallel.
	var c0, c1, c2, c3 uint64
	n := len(s)
	i := 0
	for ; i <= n-4; i += 4 {
		c0 += uint64(bits.OnesCount64(s[i]))
		c1 += uint64(bits.OnesCount64(s[i+1]))
		c2 += uint64(bits.OnesCount64(s[i+2]))
		c3 += uint64(bits.OnesCount64(s[i+3]))
	}
	cnt = c0 + c1 + c2 + c3
	for ; i < n; i++ {
		cnt += uint64(bits.OnesCount64(s[i]))
	}
	return
}

func popcntMaskSlice(s, m []uint64) (cnt uint64) {
	// The next line is to help the bounds checker, it matters!
	_ = m[len(s)-1] // BCE
	// Four independent accumulators break the dependency chain on cnt
	// so the CPU can run multiple popcounts in parallel.
	var c0, c1, c2, c3 uint64
	n := len(s)
	i := 0
	for ; i <= n-4; i += 4 {
		c0 += uint64(bits.OnesCount64(s[i] &^ m[i]))
		c1 += uint64(bits.OnesCount64(s[i+1] &^ m[i+1]))
		c2 += uint64(bits.OnesCount64(s[i+2] &^ m[i+2]))
		c3 += uint64(bits.OnesCount64(s[i+3] &^ m[i+3]))
	}
	cnt = c0 + c1 + c2 + c3
	for ; i < n; i++ {
		cnt += uint64(bits.OnesCount64(s[i] &^ m[i]))
	}
	return
}

// popcntAndSlice computes the population count of the AND of two slices.
// It assumes that len(m) >= len(s) > 0.
func popcntAndSlice(s, m []uint64) (cnt uint64) {
	// The next line is to help the bounds checker, it matters!
	_ = m[len(s)-1] // BCE
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] & m[i]))
	}
	return
}

// popcntOrSlice computes the population count of the OR of two slices.
// It assumes that len(m) >= len(s) > 0.
func popcntOrSlice(s, m []uint64) (cnt uint64) {
	// The next line is to help the bounds checker, it matters!
	_ = m[len(s)-1] // BCE
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] | m[i]))
	}
	return
}

// popcntXorSlice computes the population count of the XOR of two slices.
// It assumes that len(m) >= len(s) > 0.
func popcntXorSlice(s, m []uint64) (cnt uint64) {
	// The next line is to help the bounds checker, it matters!
	_ = m[len(s)-1] // BCE
	for i := range s {
		cnt += uint64(bits.OnesCount64(s[i] ^ m[i]))
	}
	return
}
