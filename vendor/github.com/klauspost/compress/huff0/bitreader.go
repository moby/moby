// Copyright 2018 Klaus Post. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Based on work Copyright (c) 2013, Yann Collet, released under BSD License.

package huff0

import (
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/klauspost/compress/internal/le"
)

// bitReader reads a bitstream in reverse.
// The last set bit indicates the start of the stream and is used
// for aligning the input.
type bitReaderBytes struct {
	in       []byte
	off      uint // next byte to read is at in[off - 1]
	value    uint64
	bitsRead uint8
}

// init initializes and resets the bit reader.
func (b *bitReaderBytes) init(in []byte) error {
	if len(in) < 1 {
		return errors.New("corrupt stream: too short")
	}
	b.in = in
	b.off = uint(len(in))
	// The highest bit of the last byte indicates where to start
	v := in[len(in)-1]
	if v == 0 {
		return errors.New("corrupt stream, did not find end of stream")
	}
	b.bitsRead = 64
	b.value = 0
	if len(in) >= 8 {
		b.fillFastStart()
	} else {
		b.fill()
		b.fill()
	}
	b.advance(8 - uint8(highBit32(uint32(v))))
	return nil
}

// peekByteFast requires that at least one byte is requested every time.
// There are no checks if the buffer is filled.
func (b *bitReaderBytes) peekByteFast() uint8 {
	got := uint8(b.value >> 56)
	return got
}

func (b *bitReaderBytes) advance(n uint8) {
	b.bitsRead += n
	b.value <<= n & 63
}

// fillFast() will make sure at least 32 bits are available.
// There must be at least 4 bytes available.
func (b *bitReaderBytes) fillFast() {
	if b.bitsRead < 32 {
		return
	}

	// 2 bounds checks.
	low := le.Load32(b.in, b.off-4)
	b.value |= uint64(low) << (b.bitsRead - 32)
	b.bitsRead -= 32
	b.off -= 4
}

// fillFastStart() assumes the bitReaderBytes is empty and there is at least 8 bytes to read.
func (b *bitReaderBytes) fillFastStart() {
	// Do single re-slice to avoid bounds checks.
	b.value = le.Load64(b.in, b.off-8)
	b.bitsRead = 0
	b.off -= 8
}

// fill() will make sure at least 32 bits are available.
func (b *bitReaderBytes) fill() {
	if b.bitsRead < 32 {
		return
	}
	if b.off >= 4 {
		low := le.Load32(b.in, b.off-4)
		b.value |= uint64(low) << (b.bitsRead - 32)
		b.bitsRead -= 32
		b.off -= 4
		return
	}
	for b.off > 0 {
		b.value |= uint64(b.in[b.off-1]) << (b.bitsRead - 8)
		b.bitsRead -= 8
		b.off--
	}
}

// finished returns true if all bits have been read from the bit stream.
func (b *bitReaderBytes) finished() bool {
	return b.off == 0 && b.bitsRead >= 64
}

func (b *bitReaderBytes) remaining() uint {
	return b.off*8 + uint(64-b.bitsRead)
}

// close the bitstream and returns an error if out-of-buffer reads occurred.
func (b *bitReaderBytes) close() error {
	// Release reference.
	b.in = nil
	if b.remaining() > 0 {
		return fmt.Errorf("corrupt input: %d bits remain on stream", b.remaining())
	}
	if b.bitsRead > 64 {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// bitReaderShifted reads a bitstream in reverse.
// The last set bit indicates the start of the stream and is used
// for aligning the input.
type bitReaderShifted struct {
	in       []byte
	off      uint // next byte to read is at in[off - 1]
	value    uint64
	bitsRead uint8
}

// init initializes and resets the bit reader.
func (b *bitReaderShifted) init(in []byte) error {
	if len(in) < 1 {
		return errors.New("corrupt stream: too short")
	}
	b.in = in
	b.off = uint(len(in))
	// The highest bit of the last byte indicates where to start
	v := in[len(in)-1]
	if v == 0 {
		return errors.New("corrupt stream, did not find end of stream")
	}
	b.bitsRead = 64
	b.value = 0
	if len(in) >= 8 {
		b.fillFastStart()
	} else {
		b.fill()
		b.fill()
	}
	b.advance(8 - uint8(highBit32(uint32(v))))
	return nil
}

// peekBitsFast requires that at least one bit is requested every time.
// There are no checks if the buffer is filled.
func (b *bitReaderShifted) peekBitsFast(n uint8) uint16 {
	return uint16(b.value >> ((64 - n) & 63))
}

func (b *bitReaderShifted) advance(n uint8) {
	b.bitsRead += n
	b.value <<= n & 63
}

// fillFast() will make sure at least 32 bits are available.
// There must be at least 4 bytes available.
func (b *bitReaderShifted) fillFast() {
	if b.bitsRead < 32 {
		return
	}

	low := le.Load32(b.in, b.off-4)
	b.value |= uint64(low) << ((b.bitsRead - 32) & 63)
	b.bitsRead -= 32
	b.off -= 4
}

// fillFastStart() assumes the bitReaderShifted is empty and there is at least 8 bytes to read.
func (b *bitReaderShifted) fillFastStart() {
	b.value = le.Load64(b.in, b.off-8)
	b.bitsRead = 0
	b.off -= 8
}

// fill() will make sure at least 32 bits are available.
func (b *bitReaderShifted) fill() {
	if b.bitsRead < 32 {
		return
	}
	if b.off > 4 {
		low := le.Load32(b.in, b.off-4)
		b.value |= uint64(low) << ((b.bitsRead - 32) & 63)
		b.bitsRead -= 32
		b.off -= 4
		return
	}
	for b.off > 0 {
		b.value |= uint64(b.in[b.off-1]) << ((b.bitsRead - 8) & 63)
		b.bitsRead -= 8
		b.off--
	}
}

func (b *bitReaderShifted) remaining() uint {
	return b.off*8 + uint(64-b.bitsRead)
}

// canUseAsm reports whether the reader has a full 8-byte window ahead of
// its read pointer, which the Decompress4X asm loops need to take over.
func (b *bitReaderShifted) canUseAsm() bool {
	return b.off >= 8
}

// prepareForAsm establishes the invariant the Decompress4X asm loops rely
// on: value == load64(in[off:off+8]) << bitsRead with bitsRead <= 7, so
// that a full group of symbols can never shift their sentinel bit out of
// the container. init leaves bitsRead == 8 when the final byte of the
// stream is exactly 0x01; that whole byte is consumed, so the same position
// is the window one byte lower with nothing consumed. Requires canUseAsm.
func (b *bitReaderShifted) prepareForAsm() {
	if b.bitsRead >= 8 {
		b.off--
		b.value = le.Load64(b.in, b.off)
		b.bitsRead -= 8
	}
}

// restoreFromAsm converts the state left behind by the Decompress4X asm
// loops back to the invariant the Go code relies on: value holds the 8
// bytes at in[off:off+8] shifted left by bitsRead, with the low bitsRead
// bits zero.
//
// The asm keeps a sentinel bit in value just below the unread bits, so its
// trailing zero count is the number of consumed bits. The sentinel is ORed
// over the lowest bit of the window, which the loop never consumes before
// re-reading memory, so the window is re-read here too rather than taken
// from value. The asm reports off as the signed distance from the start of
// the stream, and it can be negative: a reload always reads a whole 8-byte
// window, so a stream that is nearly drained ends with up to 7 bytes of the
// previous stream (or the jump table) below its start inside the window.
// Those bytes sit below the stream's own bits and count as consumed.
// Anything further below, or more bits consumed than the stream holds, is
// corruption.
func (b *bitReaderShifted) restoreFromAsm() error {
	off := int(b.off)
	consumed := uint(bits.TrailingZeros64(b.value))
	if off < 0 {
		if off < -7 {
			return errors.New("corruption detected: stream underrun")
		}
		consumed += uint(-off) * 8
		if consumed > 64 {
			return errors.New("corruption detected: stream underrun")
		}
		off = 0
	}
	if off+8 > len(b.in) {
		return errors.New("corruption detected: stream overrun")
	}
	b.off = uint(off)
	b.bitsRead = uint8(consumed)
	if consumed >= 64 {
		b.value = 0
	} else {
		b.value = le.Load64(b.in, b.off) << consumed
	}
	return nil
}

// close the bitstream and returns an error if out-of-buffer reads occurred.
func (b *bitReaderShifted) close() error {
	// Release reference.
	b.in = nil
	if b.remaining() > 0 {
		return fmt.Errorf("corrupt input: %d bits remain on stream", b.remaining())
	}
	if b.bitsRead > 64 {
		return io.ErrUnexpectedEOF
	}
	return nil
}
