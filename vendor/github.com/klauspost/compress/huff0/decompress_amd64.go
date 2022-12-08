//go:build amd64 && !appengine && !noasm && gc
// +build amd64,!appengine,!noasm,gc

// This file contains the specialisation of Decoder.Decompress4X
// that uses an asm implementation of its main loop.
package huff0

import (
	"errors"
	"fmt"
)

// decompress4x_main_loop_x86 is an x86 assembler implementation
// of Decompress4X when tablelog > 8.
// go:noescape
func decompress4x_main_loop_x86(pbr0, pbr1, pbr2, pbr3 *bitReaderShifted,
	peekBits uint8, buf *byte, tbl *dEntrySingle) uint8

// decompress4x_8b_loop_x86 is an x86 assembler implementation
// of Decompress4X when tablelog <= 8 which decodes 4 entries
// per loop.
// go:noescape
func decompress4x_8b_loop_x86(pbr0, pbr1, pbr2, pbr3 *bitReaderShifted,
	peekBits uint8, buf *byte, tbl *dEntrySingle) uint8

// fallback8BitSize is the size where using Go version is faster.
const fallback8BitSize = 800

// Decompress4X will decompress a 4X encoded stream.
// The length of the supplied input must match the end of a block exactly.
// The *capacity* of the dst slice must match the destination size of
// the uncompressed data exactly.
func (d *Decoder) Decompress4X(dst, src []byte) ([]byte, error) {
	if len(d.dt.single) == 0 {
		return nil, errors.New("no table loaded")
	}
	if len(src) < 6+(4*1) {
		return nil, errors.New("input too small")
	}

	use8BitTables := d.actualTableLog <= 8
	if cap(dst) < fallback8BitSize && use8BitTables {
		return d.decompress4X8bit(dst, src)
	}
	var br [4]bitReaderShifted
	// Decode "jump table"
	start := 6
	for i := 0; i < 3; i++ {
		length := int(src[i*2]) | (int(src[i*2+1]) << 8)
		if start+length >= len(src) {
			return nil, errors.New("truncated input (or invalid offset)")
		}
		err := br[i].init(src[start : start+length])
		if err != nil {
			return nil, err
		}
		start += length
	}
	err := br[3].init(src[start:])
	if err != nil {
		return nil, err
	}

	// destination, offset to match first output
	dstSize := cap(dst)
	dst = dst[:dstSize]
	out := dst
	dstEvery := (dstSize + 3) / 4

	const tlSize = 1 << tableLogMax
	const tlMask = tlSize - 1
	single := d.dt.single[:tlSize]

	// Use temp table to avoid bound checks/append penalty.
	buf := d.buffer()
	var off uint8
	var decoded int

	const debug = false

	// see: bitReaderShifted.peekBitsFast()
	peekBits := uint8((64 - d.actualTableLog) & 63)

	// Decode 2 values from each decoder/loop.
	const bufoff = 256
	for {
		if br[0].off < 4 || br[1].off < 4 || br[2].off < 4 || br[3].off < 4 {
			break
		}

		if use8BitTables {
			off = decompress4x_8b_loop_x86(&br[0], &br[1], &br[2], &br[3], peekBits, &buf[0][0], &single[0])
		} else {
			off = decompress4x_main_loop_x86(&br[0], &br[1], &br[2], &br[3], peekBits, &buf[0][0], &single[0])
		}
		if debug {
			fmt.Print("DEBUG: ")
			fmt.Printf("off=%d,", off)
			for i := 0; i < 4; i++ {
				fmt.Printf(" br[%d]={bitsRead=%d, value=%x, off=%d}",
					i, br[i].bitsRead, br[i].value, br[i].off)
			}
			fmt.Println("")
		}

		if off != 0 {
			break
		}

		if bufoff > dstEvery {
			d.bufs.Put(buf)
			return nil, errors.New("corruption detected: stream overrun 1")
		}
		copy(out, buf[0][:])
		copy(out[dstEvery:], buf[1][:])
		copy(out[dstEvery*2:], buf[2][:])
		copy(out[dstEvery*3:], buf[3][:])
		out = out[bufoff:]
		decoded += bufoff * 4
		// There must at least be 3 buffers left.
		if len(out) < dstEvery*3 {
			d.bufs.Put(buf)
			return nil, errors.New("corruption detected: stream overrun 2")
		}
	}
	if off > 0 {
		ioff := int(off)
		if len(out) < dstEvery*3+ioff {
			d.bufs.Put(buf)
			return nil, errors.New("corruption detected: stream overrun 3")
		}
		copy(out, buf[0][:off])
		copy(out[dstEvery:], buf[1][:off])
		copy(out[dstEvery*2:], buf[2][:off])
		copy(out[dstEvery*3:], buf[3][:off])
		decoded += int(off) * 4
		out = out[off:]
	}

	// Decode remaining.
	remainBytes := dstEvery - (decoded / 4)
	for i := range br {
		offset := dstEvery * i
		endsAt := offset + remainBytes
		if endsAt > len(out) {
			endsAt = len(out)
		}
		br := &br[i]
		bitsLeft := br.remaining()
		for bitsLeft > 0 {
			br.fill()
			if offset >= endsAt {
				d.bufs.Put(buf)
				return nil, errors.New("corruption detected: stream overrun 4")
			}

			// Read value and increment offset.
			val := br.peekBitsFast(d.actualTableLog)
			v := single[val&tlMask].entry
			nBits := uint8(v)
			br.advance(nBits)
			bitsLeft -= uint(nBits)
			out[offset] = uint8(v >> 8)
			offset++
		}
		if offset != endsAt {
			d.bufs.Put(buf)
			return nil, fmt.Errorf("corruption detected: short output block %d, end %d != %d", i, offset, endsAt)
		}
		decoded += offset - dstEvery*i
		err = br.close()
		if err != nil {
			return nil, err
		}
	}
	d.bufs.Put(buf)
	if dstSize != decoded {
		return nil, errors.New("corruption detected: short output block")
	}
	return dst, nil
}
