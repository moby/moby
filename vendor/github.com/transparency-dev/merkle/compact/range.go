// Copyright 2019 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package compact provides compact Merkle tree data structures.
package compact

import (
	"bytes"
	"errors"
	"fmt"
	"math/bits"
)

// HashFn computes an internal node's hash using the hashes of its child nodes.
type HashFn func(left, right []byte) []byte

// VisitFn visits the node with the specified ID and hash.
type VisitFn func(id NodeID, hash []byte)

// RangeFactory allows creating compact ranges with the specified hash
// function, which must not be nil, and must not be changed.
type RangeFactory struct {
	Hash HashFn
}

// NewRange creates a Range for [begin, end) with the given set of hashes. The
// hashes correspond to the roots of the minimal set of perfect sub-trees
// covering the [begin, end) leaves range, ordered left to right.
func (f *RangeFactory) NewRange(begin, end uint64, hashes [][]byte) (*Range, error) {
	if end < begin {
		return nil, fmt.Errorf("invalid range: end=%d, want >= %d", end, begin)
	}
	if got, want := len(hashes), RangeSize(begin, end); got != want {
		return nil, fmt.Errorf("invalid hashes: got %d values, want %d", got, want)
	}
	return &Range{f: f, begin: begin, end: end, hashes: hashes}, nil
}

// NewEmptyRange returns a new Range for an empty [begin, begin) range. The
// value of begin defines where the range will start growing from when entries
// are appended to it.
func (f *RangeFactory) NewEmptyRange(begin uint64) *Range {
	return &Range{f: f, begin: begin, end: begin}
}

// Range represents a compact Merkle tree range for leaf indices [begin, end).
//
// It contains the minimal set of perfect subtrees whose leaves comprise this
// range. The structure is efficiently mergeable with other compact ranges that
// share one of the endpoints with it.
//
// For more details, see
// https://github.com/transparency-dev/merkle/blob/main/docs/compact_ranges.md.
type Range struct {
	f      *RangeFactory
	begin  uint64
	end    uint64
	hashes [][]byte
}

// Begin returns the first index covered by the range (inclusive).
func (r *Range) Begin() uint64 {
	return r.begin
}

// End returns the last index covered by the range (exclusive).
func (r *Range) End() uint64 {
	return r.end
}

// Hashes returns sub-tree hashes corresponding to the minimal set of perfect
// sub-trees covering the [begin, end) range, ordered left to right.
func (r *Range) Hashes() [][]byte {
	return r.hashes
}

// Append extends the compact range by appending the passed in hash to it. It
// reports all the added nodes through the visitor function (if non-nil).
func (r *Range) Append(hash []byte, visitor VisitFn) error {
	if visitor != nil {
		visitor(NewNodeID(0, r.end), hash)
	}
	return r.appendImpl(r.end+1, hash, nil, visitor)
}

// AppendRange extends the compact range by merging in the other compact range
// from the right. It uses the tree hasher to calculate hashes of newly created
// nodes, and reports them through the visitor function (if non-nil).
func (r *Range) AppendRange(other *Range, visitor VisitFn) error {
	if other.f != r.f {
		return errors.New("incompatible ranges")
	}
	if got, want := other.begin, r.end; got != want {
		return fmt.Errorf("ranges are disjoint: other.begin=%d, want %d", got, want)
	}
	if len(other.hashes) == 0 { // The other range is empty, merging is trivial.
		return nil
	}
	return r.appendImpl(other.end, other.hashes[0], other.hashes[1:], visitor)
}

// GetRootHash returns the root hash of the Merkle tree represented by this
// compact range. Requires the range to start at index 0. If the range is
// empty, returns nil.
//
// If visitor is not nil, it is called with all "ephemeral" nodes (i.e. the
// ones rooting imperfect subtrees) along the right border of the tree.
func (r *Range) GetRootHash(visitor VisitFn) ([]byte, error) {
	if r.begin != 0 {
		return nil, fmt.Errorf("begin=%d, want 0", r.begin)
	}
	ln := len(r.hashes)
	if ln == 0 {
		return nil, nil
	}
	hash := r.hashes[ln-1]
	// All non-perfect subtree hashes along the right border of the tree
	// correspond to the parents of all perfect subtree nodes except the lowest
	// one (therefore the loop skips it).
	for i, size := ln-2, r.end; i >= 0; i-- {
		hash = r.f.Hash(r.hashes[i], hash)
		if visitor != nil {
			size &= size - 1                              // Delete the previous node.
			level := uint(bits.TrailingZeros64(size)) + 1 // Compute the parent level.
			index := size >> level                        // And its horizontal index.
			visitor(NewNodeID(level, index), hash)
		}
	}
	return hash, nil
}

// Equal compares two Ranges for equality.
func (r *Range) Equal(other *Range) bool {
	if r.f != other.f || r.begin != other.begin || r.end != other.end {
		return false
	}
	if len(r.hashes) != len(other.hashes) {
		return false
	}
	for i := range r.hashes {
		if !bytes.Equal(r.hashes[i], other.hashes[i]) {
			return false
		}
	}
	return true
}

// appendImpl extends the compact range by merging the [r.end, end) compact
// range into it. The other compact range is decomposed into a seed hash and
// all the other hashes (possibly none). The method uses the tree hasher to
// calculate hashes of newly created nodes, and reports them through the
// visitor function (if non-nil).
func (r *Range) appendImpl(end uint64, seed []byte, hashes [][]byte, visitor VisitFn) error {
	// Bits [low, high) of r.end encode the merge path, i.e. the sequence of node
	// merges that transforms the two compact ranges into one.
	low, high := getMergePath(r.begin, r.end, end)
	if high < low {
		high = low
	}
	index := r.end >> low
	// Now bits [0, high-low) of index encode the merge path.

	// The number of one bits in index is the number of nodes from the left range
	// that will be merged, and zero bits correspond to the nodes in the right
	// range. Below we make sure that both ranges have enough hashes, which can
	// be false only in case the data is corrupted in some way.
	ones := bits.OnesCount64(index & (1<<(high-low) - 1))
	if ln := len(r.hashes); ln < ones {
		return fmt.Errorf("corrupted lhs range: got %d hashes, want >= %d", ln, ones)
	}
	if ln, zeros := len(hashes), int(high-low)-ones; ln < zeros {
		return fmt.Errorf("corrupted rhs range: got %d hashes, want >= %d", ln+1, zeros+1)
	}

	// Some of the trailing nodes of the left compact range, and some of the
	// leading nodes of the right range, are sequentially merged with the seed,
	// according to the mask. All new nodes are reported through the visitor.
	idx1, idx2 := len(r.hashes), 0
	for h := low; h < high; h++ {
		if index&1 == 0 {
			seed = r.f.Hash(seed, hashes[idx2])
			idx2++
		} else {
			idx1--
			seed = r.f.Hash(r.hashes[idx1], seed)
		}
		index >>= 1
		if visitor != nil {
			visitor(NewNodeID(h+1, index), seed)
		}
	}

	// All nodes from both ranges that have not been merged are bundled together
	// with the "merged" seed node.
	r.hashes = append(append(r.hashes[:idx1], seed), hashes[idx2:]...)
	r.end = end
	return nil
}

// getMergePath returns the merging path between the compact range [begin, mid)
// and [mid, end). The path is represented as a range of bits within mid, with
// bit indices [low, high). A bit value of 1 on level i of mid means that the
// node on this level merges with the corresponding node in the left compact
// range, whereas 0 represents merging with the right compact range. If the
// path is empty then high <= low.
//
// The output is not specified if begin <= mid <= end doesn't hold, but the
// function never panics.
func getMergePath(begin, mid, end uint64) (uint, uint) {
	low := bits.TrailingZeros64(mid)
	high := 64
	if begin != 0 {
		high = bits.Len64(mid ^ (begin - 1))
	}
	if high2 := bits.Len64((mid - 1) ^ end); high2 < high {
		high = high2
	}
	return uint(low), uint(high - 1)
}

// Decompose splits the [begin, end) range into a minimal number of sub-ranges,
// each of which is of the form [m * 2^k, (m+1) * 2^k), i.e. of length 2^k, for
// some integers m, k >= 0.
//
// The sequence of sizes is returned encoded as bitmasks left and right, where:
//   - a 1 bit in a bitmask denotes a sub-range of the corresponding size 2^k
//   - left mask bits in LSB-to-MSB order encode the left part of the sequence
//   - right mask bits in MSB-to-LSB order encode the right part
//
// The corresponding values of m are not returned (they can be calculated from
// begin and the sub-range sizes).
//
// For example, (begin, end) values of (0b110, 0b11101) would indicate a
// sequence of tree sizes: 2,8; 8,4,1.
//
// The output is not specified if begin > end, but the function never panics.
func Decompose(begin, end uint64) (uint64, uint64) {
	// Special case, as the code below works only if begin != 0, or end < 2^63.
	if begin == 0 {
		return 0, end
	}
	xbegin := begin - 1
	// Find where paths to leaves #begin-1 and #end diverge, and mask the upper
	// bits away, as only the nodes strictly below this point are in the range.
	d := bits.Len64(xbegin^end) - 1
	mask := uint64(1)<<uint(d) - 1
	// The left part of the compact range consists of all nodes strictly below
	// and to the right from the path to leaf #begin-1, corresponding to zero
	// bits in the masked part of begin-1. Likewise, the right part consists of
	// nodes below and to the left from the path to leaf #end, corresponding to
	// ones in the masked part of end.
	return ^xbegin & mask, end & mask
}
