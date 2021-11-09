// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"fmt"
	"math/bits"
)

const (
	bestLongTableBits = 20                     // Bits used in the long match table
	bestLongTableSize = 1 << bestLongTableBits // Size of the table

	// Note: Increasing the short table bits or making the hash shorter
	// can actually lead to compression degradation since it will 'steal' more from the
	// long match table and match offsets are quite big.
	// This greatly depends on the type of input.
	bestShortTableBits = 16                      // Bits used in the short match table
	bestShortTableSize = 1 << bestShortTableBits // Size of the table
)

// bestFastEncoder uses 2 tables, one for short matches (5 bytes) and one for long matches.
// The long match table contains the previous entry with the same hash,
// effectively making it a "chain" of length 2.
// When we find a long match we choose between the two values and select the longest.
// When we find a short match, after checking the long, we check if we can find a long at n+1
// and that it is longer (lazy matching).
type bestFastEncoder struct {
	fastBase
	table         [bestShortTableSize]prevEntry
	longTable     [bestLongTableSize]prevEntry
	dictTable     []prevEntry
	dictLongTable []prevEntry
}

// Encode improves compression...
func (e *bestFastEncoder) Encode(blk *blockEnc, src []byte) {
	const (
		// Input margin is the number of bytes we read (8)
		// and the maximum we will read ahead (2)
		inputMargin            = 8 + 4
		minNonLiteralBlockSize = 16
	)

	// Protect against e.cur wraparound.
	for e.cur >= bufferReset {
		if len(e.hist) == 0 {
			for i := range e.table[:] {
				e.table[i] = prevEntry{}
			}
			for i := range e.longTable[:] {
				e.longTable[i] = prevEntry{}
			}
			e.cur = e.maxMatchOff
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - e.maxMatchOff
		for i := range e.table[:] {
			v := e.table[i].offset
			v2 := e.table[i].prev
			if v < minOff {
				v = 0
				v2 = 0
			} else {
				v = v - e.cur + e.maxMatchOff
				if v2 < minOff {
					v2 = 0
				} else {
					v2 = v2 - e.cur + e.maxMatchOff
				}
			}
			e.table[i] = prevEntry{
				offset: v,
				prev:   v2,
			}
		}
		for i := range e.longTable[:] {
			v := e.longTable[i].offset
			v2 := e.longTable[i].prev
			if v < minOff {
				v = 0
				v2 = 0
			} else {
				v = v - e.cur + e.maxMatchOff
				if v2 < minOff {
					v2 = 0
				} else {
					v2 = v2 - e.cur + e.maxMatchOff
				}
			}
			e.longTable[i] = prevEntry{
				offset: v,
				prev:   v2,
			}
		}
		e.cur = e.maxMatchOff
		break
	}

	s := e.addBlock(src)
	blk.size = len(src)
	if len(src) < minNonLiteralBlockSize {
		blk.extraLits = len(src)
		blk.literals = blk.literals[:len(src)]
		copy(blk.literals, src)
		return
	}

	// Override src
	src = e.hist
	sLimit := int32(len(src)) - inputMargin
	const kSearchStrength = 10

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := s
	cv := load6432(src, s)

	// Relative offsets
	offset1 := int32(blk.recentOffsets[0])
	offset2 := int32(blk.recentOffsets[1])
	offset3 := int32(blk.recentOffsets[2])

	addLiterals := func(s *seq, until int32) {
		if until == nextEmit {
			return
		}
		blk.literals = append(blk.literals, src[nextEmit:until]...)
		s.litLen = uint32(until - nextEmit)
	}
	_ = addLiterals

	if debug {
		println("recent offsets:", blk.recentOffsets)
	}

encodeLoop:
	for {
		// We allow the encoder to optionally turn off repeat offsets across blocks
		canRepeat := len(blk.sequences) > 2

		if debugAsserts && canRepeat && offset1 == 0 {
			panic("offset0 was 0")
		}

		type match struct {
			offset int32
			s      int32
			length int32
			rep    int32
		}
		matchAt := func(offset int32, s int32, first uint32, rep int32) match {
			if s-offset >= e.maxMatchOff || load3232(src, offset) != first {
				return match{offset: offset, s: s}
			}
			return match{offset: offset, s: s, length: 4 + e.matchlen(s+4, offset+4, src), rep: rep}
		}

		bestOf := func(a, b match) match {
			aScore := b.s - a.s + a.length
			bScore := a.s - b.s + b.length
			if a.rep < 0 {
				aScore = aScore - int32(bits.Len32(uint32(a.offset)))/8
			}
			if b.rep < 0 {
				bScore = bScore - int32(bits.Len32(uint32(b.offset)))/8
			}
			if aScore >= bScore {
				return a
			}
			return b
		}
		const goodEnough = 100

		nextHashL := hash8(cv, bestLongTableBits)
		nextHashS := hash4x64(cv, bestShortTableBits)
		candidateL := e.longTable[nextHashL]
		candidateS := e.table[nextHashS]

		best := bestOf(matchAt(candidateL.offset-e.cur, s, uint32(cv), -1), matchAt(candidateL.prev-e.cur, s, uint32(cv), -1))
		best = bestOf(best, matchAt(candidateS.offset-e.cur, s, uint32(cv), -1))
		best = bestOf(best, matchAt(candidateS.prev-e.cur, s, uint32(cv), -1))
		if canRepeat && best.length < goodEnough {
			best = bestOf(best, matchAt(s-offset1+1, s+1, uint32(cv>>8), 1))
			best = bestOf(best, matchAt(s-offset2+1, s+1, uint32(cv>>8), 2))
			best = bestOf(best, matchAt(s-offset3+1, s+1, uint32(cv>>8), 3))
			if best.length > 0 {
				best = bestOf(best, matchAt(s-offset1+3, s+3, uint32(cv>>24), 1))
				best = bestOf(best, matchAt(s-offset2+3, s+3, uint32(cv>>24), 2))
				best = bestOf(best, matchAt(s-offset3+3, s+3, uint32(cv>>24), 3))
			}
		}
		// Load next and check...
		e.longTable[nextHashL] = prevEntry{offset: s + e.cur, prev: candidateL.offset}
		e.table[nextHashS] = prevEntry{offset: s + e.cur, prev: candidateS.offset}

		// Look far ahead, unless we have a really long match already...
		if best.length < goodEnough {
			// No match found, move forward on input, no need to check forward...
			if best.length < 4 {
				s += 1 + (s-nextEmit)>>(kSearchStrength-1)
				if s >= sLimit {
					break encodeLoop
				}
				cv = load6432(src, s)
				continue
			}

			s++
			candidateS = e.table[hash4x64(cv>>8, bestShortTableBits)]
			cv = load6432(src, s)
			cv2 := load6432(src, s+1)
			candidateL = e.longTable[hash8(cv, bestLongTableBits)]
			candidateL2 := e.longTable[hash8(cv2, bestLongTableBits)]

			best = bestOf(best, matchAt(candidateS.offset-e.cur, s, uint32(cv), -1))
			best = bestOf(best, matchAt(candidateL.offset-e.cur, s, uint32(cv), -1))
			best = bestOf(best, matchAt(candidateL.prev-e.cur, s, uint32(cv), -1))
			best = bestOf(best, matchAt(candidateL2.offset-e.cur, s+1, uint32(cv2), -1))
			best = bestOf(best, matchAt(candidateL2.prev-e.cur, s+1, uint32(cv2), -1))
		}

		// We have a match, we can store the forward value
		if best.rep > 0 {
			s = best.s
			var seq seq
			seq.matchLen = uint32(best.length - zstdMinMatch)

			// We might be able to match backwards.
			// Extend as long as we can.
			start := best.s
			// We end the search early, so we don't risk 0 literals
			// and have to do special offset treatment.
			startLimit := nextEmit + 1

			tMin := s - e.maxMatchOff
			if tMin < 0 {
				tMin = 0
			}
			repIndex := best.offset
			for repIndex > tMin && start > startLimit && src[repIndex-1] == src[start-1] && seq.matchLen < maxMatchLength-zstdMinMatch-1 {
				repIndex--
				start--
				seq.matchLen++
			}
			addLiterals(&seq, start)

			// rep 0
			seq.offset = uint32(best.rep)
			if debugSequences {
				println("repeat sequence", seq, "next s:", s)
			}
			blk.sequences = append(blk.sequences, seq)

			// Index match start+1 (long) -> s - 1
			index0 := s
			s = best.s + best.length

			nextEmit = s
			if s >= sLimit {
				if debug {
					println("repeat ended", s, best.length)

				}
				break encodeLoop
			}
			// Index skipped...
			off := index0 + e.cur
			for index0 < s-1 {
				cv0 := load6432(src, index0)
				h0 := hash8(cv0, bestLongTableBits)
				h1 := hash4x64(cv0, bestShortTableBits)
				e.longTable[h0] = prevEntry{offset: off, prev: e.longTable[h0].offset}
				e.table[h1] = prevEntry{offset: off, prev: e.table[h1].offset}
				off++
				index0++
			}
			switch best.rep {
			case 2:
				offset1, offset2 = offset2, offset1
			case 3:
				offset1, offset2, offset3 = offset3, offset1, offset2
			}
			cv = load6432(src, s)
			continue
		}

		// A 4-byte match has been found. Update recent offsets.
		// We'll later see if more than 4 bytes.
		s = best.s
		t := best.offset
		offset1, offset2, offset3 = s-t, offset1, offset2

		if debugAsserts && s <= t {
			panic(fmt.Sprintf("s (%d) <= t (%d)", s, t))
		}

		if debugAsserts && canRepeat && int(offset1) > len(src) {
			panic("invalid offset")
		}

		// Extend the n-byte match as long as possible.
		l := best.length

		// Extend backwards
		tMin := s - e.maxMatchOff
		if tMin < 0 {
			tMin = 0
		}
		for t > tMin && s > nextEmit && src[t-1] == src[s-1] && l < maxMatchLength {
			s--
			t--
			l++
		}

		// Write our sequence
		var seq seq
		seq.litLen = uint32(s - nextEmit)
		seq.matchLen = uint32(l - zstdMinMatch)
		if seq.litLen > 0 {
			blk.literals = append(blk.literals, src[nextEmit:s]...)
		}
		seq.offset = uint32(s-t) + 3
		s += l
		if debugSequences {
			println("sequence", seq, "next s:", s)
		}
		blk.sequences = append(blk.sequences, seq)
		nextEmit = s
		if s >= sLimit {
			break encodeLoop
		}

		// Index match start+1 (long) -> s - 1
		index0 := s - l + 1
		// every entry
		for index0 < s-1 {
			cv0 := load6432(src, index0)
			h0 := hash8(cv0, bestLongTableBits)
			h1 := hash4x64(cv0, bestShortTableBits)
			off := index0 + e.cur
			e.longTable[h0] = prevEntry{offset: off, prev: e.longTable[h0].offset}
			e.table[h1] = prevEntry{offset: off, prev: e.table[h1].offset}
			index0++
		}

		cv = load6432(src, s)
		if !canRepeat {
			continue
		}

		// Check offset 2
		for {
			o2 := s - offset2
			if load3232(src, o2) != uint32(cv) {
				// Do regular search
				break
			}

			// Store this, since we have it.
			nextHashS := hash4x64(cv, bestShortTableBits)
			nextHashL := hash8(cv, bestLongTableBits)

			// We have at least 4 byte match.
			// No need to check backwards. We come straight from a match
			l := 4 + e.matchlen(s+4, o2+4, src)

			e.longTable[nextHashL] = prevEntry{offset: s + e.cur, prev: e.longTable[nextHashL].offset}
			e.table[nextHashS] = prevEntry{offset: s + e.cur, prev: e.table[nextHashS].offset}
			seq.matchLen = uint32(l) - zstdMinMatch
			seq.litLen = 0

			// Since litlen is always 0, this is offset 1.
			seq.offset = 1
			s += l
			nextEmit = s
			if debugSequences {
				println("sequence", seq, "next s:", s)
			}
			blk.sequences = append(blk.sequences, seq)

			// Swap offset 1 and 2.
			offset1, offset2 = offset2, offset1
			if s >= sLimit {
				// Finished
				break encodeLoop
			}
			cv = load6432(src, s)
		}
	}

	if int(nextEmit) < len(src) {
		blk.literals = append(blk.literals, src[nextEmit:]...)
		blk.extraLits = len(src) - int(nextEmit)
	}
	blk.recentOffsets[0] = uint32(offset1)
	blk.recentOffsets[1] = uint32(offset2)
	blk.recentOffsets[2] = uint32(offset3)
	if debug {
		println("returning, recent offsets:", blk.recentOffsets, "extra literals:", blk.extraLits)
	}
}

// EncodeNoHist will encode a block with no history and no following blocks.
// Most notable difference is that src will not be copied for history and
// we do not need to check for max match length.
func (e *bestFastEncoder) EncodeNoHist(blk *blockEnc, src []byte) {
	e.ensureHist(len(src))
	e.Encode(blk, src)
}

// ResetDict will reset and set a dictionary if not nil
func (e *bestFastEncoder) Reset(d *dict, singleBlock bool) {
	e.resetBase(d, singleBlock)
	if d == nil {
		return
	}
	// Init or copy dict table
	if len(e.dictTable) != len(e.table) || d.id != e.lastDictID {
		if len(e.dictTable) != len(e.table) {
			e.dictTable = make([]prevEntry, len(e.table))
		}
		end := int32(len(d.content)) - 8 + e.maxMatchOff
		for i := e.maxMatchOff; i < end; i += 4 {
			const hashLog = bestShortTableBits

			cv := load6432(d.content, i-e.maxMatchOff)
			nextHash := hash4x64(cv, hashLog)      // 0 -> 4
			nextHash1 := hash4x64(cv>>8, hashLog)  // 1 -> 5
			nextHash2 := hash4x64(cv>>16, hashLog) // 2 -> 6
			nextHash3 := hash4x64(cv>>24, hashLog) // 3 -> 7
			e.dictTable[nextHash] = prevEntry{
				prev:   e.dictTable[nextHash].offset,
				offset: i,
			}
			e.dictTable[nextHash1] = prevEntry{
				prev:   e.dictTable[nextHash1].offset,
				offset: i + 1,
			}
			e.dictTable[nextHash2] = prevEntry{
				prev:   e.dictTable[nextHash2].offset,
				offset: i + 2,
			}
			e.dictTable[nextHash3] = prevEntry{
				prev:   e.dictTable[nextHash3].offset,
				offset: i + 3,
			}
		}
		e.lastDictID = d.id
	}

	// Init or copy dict table
	if len(e.dictLongTable) != len(e.longTable) || d.id != e.lastDictID {
		if len(e.dictLongTable) != len(e.longTable) {
			e.dictLongTable = make([]prevEntry, len(e.longTable))
		}
		if len(d.content) >= 8 {
			cv := load6432(d.content, 0)
			h := hash8(cv, bestLongTableBits)
			e.dictLongTable[h] = prevEntry{
				offset: e.maxMatchOff,
				prev:   e.dictLongTable[h].offset,
			}

			end := int32(len(d.content)) - 8 + e.maxMatchOff
			off := 8 // First to read
			for i := e.maxMatchOff + 1; i < end; i++ {
				cv = cv>>8 | (uint64(d.content[off]) << 56)
				h := hash8(cv, bestLongTableBits)
				e.dictLongTable[h] = prevEntry{
					offset: i,
					prev:   e.dictLongTable[h].offset,
				}
				off++
			}
		}
		e.lastDictID = d.id
	}
	// Reset table to initial state
	copy(e.longTable[:], e.dictLongTable)

	e.cur = e.maxMatchOff
	// Reset table to initial state
	copy(e.table[:], e.dictTable)
}
