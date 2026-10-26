// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"math"
	"math/bits"
)

const (
	maxBitsLimit = 16
	// number of valid literals
	literalCount = 286
)

// hcode is a huffman code with a bit code and bit length.
type hcode uint32

func (h hcode) len() uint8 {
	return uint8(h)
}

func (h hcode) code64() uint64 {
	return uint64(h >> 8)
}

func (h hcode) zero() bool {
	return h == 0
}

type huffmanEncoder struct {
	codes    []hcode
	bitCount [17]int32

	// Allocate a reusable buffer with the longest possible frequency table.
	// Possible lengths are codegenCodeCount, offsetCodeCount and literalCount.
	// The largest of these is literalCount, so we allocate for that case.
	freqcache [literalCount + 1]literalNode

	// sortcache is scratch space for sortByFreq.
	sortcache [literalCount + 1]literalNode
}

type literalNode struct {
	literal uint16
	freq    uint16
}

// A levelInfo describes the state of the constructed tree for a given depth.
type levelInfo struct {
	// Our level.  for better printing
	level int32

	// The frequency of the last node at this level
	lastFreq int32

	// The frequency of the next character to add to this level
	nextCharFreq int32

	// The frequency of the next pair (from level below) to add to this level.
	// Only valid if the "needed" value of the next lower level is 0.
	nextPairFreq int32

	// The number of chains remaining to generate for this level before moving
	// up to the next level
	needed int32
}

// set sets the code and length of an hcode.
func (h *hcode) set(code uint16, length uint8) {
	*h = hcode(length) | (hcode(code) << 8)
}

func newhcode(code uint16, length uint8) hcode {
	return hcode(length) | (hcode(code) << 8)
}

func reverseBits(number uint16, bitLength byte) uint16 {
	return bits.Reverse16(number << ((16 - bitLength) & 15))
}

func maxNode() literalNode { return literalNode{math.MaxUint16, math.MaxUint16} }

func newHuffmanEncoder(size int) *huffmanEncoder {
	// Make capacity to next power of two.
	c := uint(bits.Len32(uint32(size - 1)))
	return &huffmanEncoder{codes: make([]hcode, size, 1<<c)}
}

// Generates a HuffmanCode corresponding to the fixed literal table
func generateFixedLiteralEncoding() *huffmanEncoder {
	h := newHuffmanEncoder(literalCount)
	codes := h.codes
	var ch uint16
	for ch = range uint16(literalCount) {
		var bits uint16
		var size uint8
		switch {
		case ch < 144:
			// size 8, 000110000  .. 10111111
			bits = ch + 48
			size = 8
		case ch < 256:
			// size 9, 110010000 .. 111111111
			bits = ch + 400 - 144
			size = 9
		case ch < 280:
			// size 7, 0000000 .. 0010111
			bits = ch - 256
			size = 7
		default:
			// size 8, 11000000 .. 11000111
			bits = ch + 192 - 280
			size = 8
		}
		codes[ch] = newhcode(reverseBits(bits, size), size)
	}
	return h
}

func generateFixedOffsetEncoding() *huffmanEncoder {
	h := newHuffmanEncoder(30)
	codes := h.codes
	for ch := range codes {
		codes[ch] = newhcode(reverseBits(uint16(ch), 5), 5)
	}
	return h
}

var fixedLiteralEncoding = generateFixedLiteralEncoding()
var fixedOffsetEncoding = generateFixedOffsetEncoding()

func (h *huffmanEncoder) bitLength(freq []uint16) int {
	var total int
	for i, f := range freq {
		if f != 0 {
			total += int(f) * int(h.codes[i].len())
		}
	}
	return total
}

// canReuseBits returns the number of bits or math.MaxInt32 if the encoder cannot be reused.
func (h *huffmanEncoder) canReuseBits(freq []uint16) int {
	var total int
	for i, f := range freq {
		if f != 0 {
			code := h.codes[i]
			if code.zero() {
				return math.MaxInt32
			}
			total += int(f) * int(code.len())
		}
	}
	return total
}

// Return the number of literals assigned to each bit size in the Huffman encoding
//
// This method is only called when list.length >= 3
// The cases of 0, 1, and 2 literals are handled by special case code.
//
// list  An array of the literals with non-zero frequencies
//
//	and their associated frequencies. The array is in order of increasing
//	frequency, and has as its last element a special element with frequency
//	MaxInt32
//
// maxBits     The maximum number of bits that should be used to encode any literal.
//
//	Must be less than 16.
//
// return      An integer array in which array[i] indicates the number of literals
//
//	that should be encoded in i bits.
func (h *huffmanEncoder) bitCounts(list []literalNode, maxBits int32) []int32 {
	if maxBits >= maxBitsLimit {
		panic("flate: maxBits too large")
	}
	n := int32(len(list))
	list = list[0 : n+1]
	list[n] = maxNode()

	// The tree can't have greater depth than n - 1, no matter what. This
	// saves a little bit of work in some small cases
	if maxBits > n-1 {
		maxBits = n - 1
	}

	// Create information about each of the levels.
	// A bogus "Level 0" whose sole purpose is so that
	// level1.prev.needed==0.  This makes level1.nextPairFreq
	// be a legitimate value that never gets chosen.
	var levels [maxBitsLimit]levelInfo
	// leafCounts[i] counts the number of literals at the left
	// of ancestors of the rightmost node at level i.
	// leafCounts[i][j] is the number of literals at the left
	// of the level j ancestor.
	var leafCounts [maxBitsLimit][maxBitsLimit]int32

	// Descending to only have 1 bounds check.
	l2f := int32(list[2].freq)
	l1f := int32(list[1].freq)
	l0f := int32(list[0].freq) + int32(list[1].freq)

	for level := int32(1); level <= maxBits; level++ {
		// For every level, the first two items are the first two characters.
		// We initialize the levels as if we had already figured this out.
		levels[level] = levelInfo{
			level:        level,
			lastFreq:     l1f,
			nextCharFreq: l2f,
			nextPairFreq: l0f,
		}
		leafCounts[level][level] = 2
		if level == 1 {
			levels[level].nextPairFreq = math.MaxInt32
		}
	}

	// We need a total of 2*n - 2 items at top level and have already generated 2.
	levels[maxBits].needed = 2*n - 4

	level := uint32(maxBits)
	for level < 16 {
		l := &levels[level]
		if l.nextPairFreq == math.MaxInt32 && l.nextCharFreq == math.MaxInt32 {
			// We've run out of both leafs and pairs.
			// End all calculations for this level.
			// To make sure we never come back to this level or any lower level,
			// set nextPairFreq impossibly large.
			l.needed = 0
			levels[level+1].nextPairFreq = math.MaxInt32
			level++
			continue
		}

		prevFreq := l.lastFreq
		if l.nextCharFreq < l.nextPairFreq {
			// The next item on this row is a leaf node.
			n := leafCounts[level][level] + 1
			l.lastFreq = l.nextCharFreq
			// Lower leafCounts are the same of the previous node.
			leafCounts[level][level] = n
			e := list[n]
			if e.literal < math.MaxUint16 {
				l.nextCharFreq = int32(e.freq)
			} else {
				l.nextCharFreq = math.MaxInt32
			}
		} else {
			// The next item on this row is a pair from the previous row.
			// nextPairFreq isn't valid until we generate two
			// more values in the level below
			l.lastFreq = l.nextPairFreq
			// Take leaf counts from the lower level, except counts[level] remains the same.
			if true {
				save := leafCounts[level][level]
				leafCounts[level] = leafCounts[level-1]
				leafCounts[level][level] = save
			} else {
				copy(leafCounts[level][:level], leafCounts[level-1][:level])
			}
			levels[l.level-1].needed = 2
		}

		if l.needed--; l.needed == 0 {
			// We've done everything we need to do for this level.
			// Continue calculating one level up. Fill in nextPairFreq
			// of that level with the sum of the two nodes we've just calculated on
			// this level.
			if l.level == maxBits {
				// All done!
				break
			}
			levels[l.level+1].nextPairFreq = prevFreq + l.lastFreq
			level++
		} else {
			// If we stole from below, move down temporarily to replenish it.
			for levels[level-1].needed > 0 {
				level--
			}
		}
	}

	// Somethings is wrong if at the end, the top level is null or hasn't used
	// all of the leaves.
	if leafCounts[maxBits][maxBits] != n {
		panic("leafCounts[maxBits][maxBits] != n")
	}

	bitCount := h.bitCount[:maxBits+1]
	bits := 1
	counts := &leafCounts[maxBits]
	for level := maxBits; level > 0; level-- {
		// chain.leafCount gives the number of literals requiring at least "bits"
		// bits to encode.
		bitCount[bits] = counts[level] - counts[level-1]
		bits++
	}
	return bitCount
}

// Look at the leaves and assign them a bit count and an encoding as specified
// in RFC 1951 3.2.2.
// list must be sorted by increasing frequency and codes must have
// all entries for literals not in list set to zero.
func (h *huffmanEncoder) assignEncodingAndSize(bitCount []int32, list []literalNode, codes []hcode) {
	// The literals list[len(list)-bits] .. list[len(list)-1]
	// are encoded using "bits" bits. Record the length of each literal
	// temporarily in codes.
	for n, bits := range bitCount {
		if n == 0 || bits == 0 {
			continue
		}
		for _, node := range list[len(list)-int(bits):] {
			codes[node.literal] = hcode(n)
		}
		list = list[0 : len(list)-int(bits)]
	}

	// Code values are assigned in literal order (not frequency order)
	// within each code length, starting from the first code of each length.
	var nextCode [maxBitsLimit]uint16
	code := uint16(0)
	for n := 1; n < len(bitCount); n++ {
		code = (code + uint16(bitCount[n-1])) << 1
		nextCode[n] = code
	}
	for i, c := range codes {
		if c == 0 {
			continue
		}
		// n is at most maxBitsLimit-1; the mask only elides bounds checks.
		n := uint8(c)
		codes[i] = newhcode(reverseBits(nextCode[n&15], n), n)
		nextCode[n&15]++
	}
}

// sortByFreq sorts list by increasing frequency.
// The sort is stable, so nodes with equal frequencies keep their order.
func (h *huffmanEncoder) sortByFreq(list []literalNode) {
	if len(list) <= 32 {
		// Insertion sort is faster for small lists.
		for i := 1; i < len(list); i++ {
			n := list[i]
			j := i
			for ; j > 0 && list[j-1].freq > n.freq; j-- {
				list[j] = list[j-1]
			}
			list[j] = n
		}
		return
	}

	// Counting sort on the low byte and then on the high byte
	// of the frequency. Both passes are stable.
	tmp := h.sortcache[:len(list)]
	var count [256]uint16
	var maxFreq uint16
	for _, n := range list {
		count[n.freq&0xff]++
		maxFreq |= n.freq
	}
	var pos uint16
	for i, c := range count {
		count[i] = pos
		pos += c
	}
	for _, n := range list {
		tmp[count[n.freq&0xff]] = n
		count[n.freq&0xff]++
	}
	if maxFreq < 256 {
		copy(list, tmp)
		return
	}
	count = [256]uint16{}
	for _, n := range tmp {
		count[n.freq>>8]++
	}
	pos = 0
	for i, c := range count {
		count[i] = pos
		pos += c
	}
	for _, n := range tmp {
		list[count[n.freq>>8]] = n
		count[n.freq>>8]++
	}
}

// Update this Huffman Code object to be the minimum code for the specified frequency count.
//
// freq  An array of frequencies, in which frequency[i] gives the frequency of literal i.
// maxBits  The maximum number of bits to use for any literal.
func (h *huffmanEncoder) generate(freq []uint16, maxBits int32) {
	list := h.freqcache[:len(freq)+1]
	codes := h.codes[:len(freq)]
	// Number of non-zero literals
	count := 0
	// Set list to be the set of all non-zero literals and their frequencies
	for i, f := range freq {
		if f != 0 {
			list[count] = literalNode{uint16(i), f}
			count++
		} else {
			codes[i] = 0
		}
	}
	list[count] = literalNode{}

	list = list[:count]
	if count <= 2 {
		// Handle the small cases here, because they are awkward for the general case code. With
		// two or fewer literals, everything has bit length 1.
		for i, node := range list {
			// "list" is in order of increasing literal value.
			h.codes[node.literal].set(uint16(i), 1)
		}
		return
	}
	// list is in order of increasing literal value, so a stable sort
	// by frequency orders it by (frequency, literal).
	h.sortByFreq(list)

	// Get the number of literals for each bit count
	bitCount := h.bitCounts(list, maxBits)
	// And do the assignment
	h.assignEncodingAndSize(bitCount, list, codes)
}

// atLeastOne clamps the result between 1 and 15.
func atLeastOne(v float32) float32 {
	if v < 1 {
		return 1
	}
	if v > 15 {
		return 15
	}
	return v
}

func histogram(b []byte, h []uint16) {
	if true && len(b) >= 8<<10 {
		// Split for bigger inputs
		histogramSplit(b, h)
	} else {
		h = h[:256]
		for _, t := range b {
			h[t]++
		}
	}
}

func histogramSplit(b []byte, h []uint16) {
	// Walk four quarters in parallel, each into its own table.
	// Tested to be faster than walking halves. Using separate tables
	// avoids the load/store dependencies that arise when two of the
	// streams hit the same byte value, which is common for text.
	var h0, h1, h2, h3 [256]uint16
	// Make size divisible by 4
	for len(b)&3 != 0 {
		h0[b[0]]++
		b = b[1:]
	}
	n := len(b) / 4
	x, y, z, w := b[:n], b[n:], b[n+n:], b[n+n+n:]
	y, z, w = y[:len(x)], z[:len(x)], w[:len(x)]
	for i, t := range x {
		h0[t]++
		h1[y[i]]++
		h2[z[i]]++
		h3[w[i]]++
	}
	h = h[:256]
	for i := range h {
		h[i] += h0[i] + h1[i] + h2[i] + h3[i]
	}
}
