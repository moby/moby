// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar

import (
	"math"
	"math/rand/v2"
)

// nextTracker tracks the next measurement that should be sampled using Algorithm L.
type nextTracker struct {
	// count is the number of measurements seen.
	count int64
	// next is the next count that will store a measurement at a random index
	// once the reservoir has been filled.
	next int64
	// w is the largest random number in a distribution that is used to compute
	// the next next.
	w float64
	// k is the size of the reservoir.
	k int
}

// reset resets the tracker to the initial state for a reservoir of size k.
func (t *nextTracker) reset() {
	t.count = 0
	if t.k <= 0 {
		return
	}
	// Random index inserts should only happen after the storage is full.
	// -1 accounts for the fact that advance() will unconditionally add 1 to
	// t.next below.
	t.next = int64(t.k) - 1

	// Initial random number in the series used to generate t.next.
	//
	// This is set before t.advance to reset or initialize the random number
	// series. Without doing so it would always be 0 or never restart a new
	// random number series.
	//
	// This maps the uniform random number in (0,1) to a geometric distribution
	// over the same interval. The mean of the distribution is inversely
	// proportional to the storage capacity.
	t.w = math.Exp(math.Log(randomFloat64()) / float64(t.k))
	t.advance()
}

// advance updates the count at which the offered measurement will overwrite an
// existing exemplar.
func (t *nextTracker) advance() {
	// Use the current random number in the series to calculate the count of the
	// next measurement that will be stored.
	//
	// Given 0 < t.w < 1, each iteration will result in subsequent t.w being
	// smaller. This translates here into the next next being selected against
	// a distribution with a higher mean (i.e. the expected value will increase
	// and replacements become less likely)
	//
	// Important to note, the new t.next will always be at least 1 more than
	// the last t.next.
	t.next += int64(math.Log(randomFloat64())/math.Log(1-t.w)) + 1

	// Calculate the next value in the random number series.
	//
	// The current value of t.w is based on the max of a distribution of random
	// numbers (i.e. `w = max(u_1,u_2,...,u_k)` for `k` equal to the capacity
	// of the storage and each `u` in the interval (0,w)). To calculate the
	// next t.w we use the fact that when the next exemplar is selected to be
	// included in the storage an existing one will be dropped, and the
	// corresponding random number in the set used to calculate t.w will also
	// be replaced. The replacement random number will also be within (0,w),
	// therefore the next t.w will be based on the same distribution (i.e.
	// `max(u_1,u_2,...,u_k)`). Therefore, we can sample the next t.w by
	// computing the next random number `u` and take t.w as `w * u^(1/k)`.
	t.w *= math.Exp(math.Log(randomFloat64()) / float64(t.k))
}

// shouldSample returns true if the measurement should be sampled.
// It also returns the index at which the measurement should be stored.
//
// The following algorithm is "Algorithm L" from Li, Kim-Hung (4 December
// 1994). "Reservoir-Sampling Algorithms of Time Complexity
// O(n(1+log(N/n)))". ACM Transactions on Mathematical Software. 20 (4):
// 481–493 (https://dl.acm.org/doi/10.1145/198429.198435).
//
// A high-level overview of "Algorithm L":
//  0. Pre-calculate the random count greater than the storage size when
//     an exemplar will be replaced.
//  1. Accept all measurements offered until the configured storage size is
//     reached.
//  2. Loop:
//     a) When the pre-calculate count is reached, replace a random
//     existing exemplar with the offered measurement.
//     b) Calculate the next random count greater than the existing one
//     which will replace another exemplars
//
// The way a "replacement" count is computed is by looking at `n` number of
// independent random numbers each corresponding to an offered measurement.
// Of these numbers the smallest `k` (the same size as the storage
// capacity) of them are kept as a subset. The maximum value in this
// subset, called `w` is used to weight another random number generation
// for the next count that will be considered.
//
// By weighting the next count computation like described, it is able to
// perform a uniformly-weighted sampling algorithm based on the number of
// samples the reservoir has seen so far. The sampling will "slow down" as
// more and more samples are offered so as to reduce a bias towards those
// offered just prior to the end of the collection.
//
// This algorithm is preferred because of its balance of simplicity and
// performance. It will compute three random numbers (the bulk of
// computation time) for each item that becomes part of the reservoir, but
// it does not spend any time on items that do not. In particular it has an
// asymptotic runtime of O(k(1 + log(n/k))) where n is the number of
// measurements offered and k is the reservoir size.
//
// See https://en.wikipedia.org/wiki/Reservoir_sampling for an overview of
// this and other reservoir sampling algorithms. See
// https://github.com/MrAlias/reservoir-sampling for a performance
// comparison of reservoir sampling algorithms.
func (t *nextTracker) shouldSample() (bool, int) {
	if t.k <= 0 {
		return false, 0
	}
	if t.count < int64(t.k) {
		idx := int(t.count)
		t.count++
		return true, idx
	}
	if t.count == t.next {
		idx := 0
		if t.k > 1 {
			idx = int(rand.Int64N(int64(t.k)))
		}
		t.advance()
		t.count++
		return true, idx
	}
	t.count++
	return false, 0
}

// randomFloat64 returns a pseudo-random value uniformly selected from
// {k / 2^53 | 1 <= k < 2^53}.
func randomFloat64() float64 {
	// rand.Float64 returns a value in [0, 1). Retry in the extremely
	// unlikely event that it returns zero to produce a value in (0, 1).
	f := rand.Float64()
	for f == 0 {
		f = rand.Float64()
	}
	return f
}
