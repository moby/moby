// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"

	"github.com/klauspost/compress/zstd/internal/xxhash"
)

type frameDec struct {
	o   decoderOptions
	crc *xxhash.Digest

	WindowSize uint64

	// Frame history passed between blocks
	history history

	rawInput byteBuffer

	// Byte buffer that can be reused for small input blocks.
	bBuf byteBuf

	FrameContentSize uint64

	DictionaryID  *uint32
	HasCheckSum   bool
	SingleSegment bool
}

const (
	// MinWindowSize is the minimum Window Size, which is 1 KB.
	MinWindowSize = 1 << 10

	// MaxWindowSize is the maximum encoder window size
	// and the default decoder maximum window size.
	MaxWindowSize = 1 << 29
)

var (
	frameMagic          = []byte{0x28, 0xb5, 0x2f, 0xfd}
	skippableFrameMagic = []byte{0x2a, 0x4d, 0x18}
)

func newFrameDec(o decoderOptions) *frameDec {
	if o.maxWindowSize > o.maxDecodedSize {
		o.maxWindowSize = o.maxDecodedSize
	}
	d := frameDec{
		o: o,
	}
	return &d
}

// reset will read the frame header and prepare for block decoding.
// If nothing can be read from the input, io.EOF will be returned.
// Any other error indicated that the stream contained data, but
// there was a problem.
func (d *frameDec) reset(br byteBuffer) error {
	d.HasCheckSum = false
	d.WindowSize = 0
	var signature [4]byte
	for {
		var err error
		// Check if we can read more...
		b, err := br.readSmall(1)
		switch err {
		case io.EOF, io.ErrUnexpectedEOF:
			return io.EOF
		default:
			return err
		case nil:
			signature[0] = b[0]
		}
		// Read the rest, don't allow io.ErrUnexpectedEOF
		b, err = br.readSmall(3)
		switch err {
		case io.EOF:
			return io.EOF
		default:
			return err
		case nil:
			copy(signature[1:], b)
		}

		if !bytes.Equal(signature[1:4], skippableFrameMagic) || signature[0]&0xf0 != 0x50 {
			if debugDecoder {
				println("Not skippable", hex.EncodeToString(signature[:]), hex.EncodeToString(skippableFrameMagic))
			}
			// Break if not skippable frame.
			break
		}
		// Read size to skip
		b, err = br.readSmall(4)
		if err != nil {
			if debugDecoder {
				println("Reading Frame Size", err)
			}
			return err
		}
		n := uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24)
		println("Skipping frame with", n, "bytes.")
		err = br.skipN(int(n))
		if err != nil {
			if debugDecoder {
				println("Reading discarded frame", err)
			}
			return err
		}
	}
	if !bytes.Equal(signature[:], frameMagic) {
		if debugDecoder {
			println("Got magic numbers: ", signature, "want:", frameMagic)
		}
		return ErrMagicMismatch
	}

	// Read Frame_Header_Descriptor
	fhd, err := br.readByte()
	if err != nil {
		if debugDecoder {
			println("Reading Frame_Header_Descriptor", err)
		}
		return err
	}
	d.SingleSegment = fhd&(1<<5) != 0

	if fhd&(1<<3) != 0 {
		return errors.New("reserved bit set on frame header")
	}

	// Read Window_Descriptor
	// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#window_descriptor
	d.WindowSize = 0
	if !d.SingleSegment {
		wd, err := br.readByte()
		if err != nil {
			if debugDecoder {
				println("Reading Window_Descriptor", err)
			}
			return err
		}
		printf("raw: %x, mantissa: %d, exponent: %d\n", wd, wd&7, wd>>3)
		windowLog := 10 + (wd >> 3)
		windowBase := uint64(1) << windowLog
		windowAdd := (windowBase / 8) * uint64(wd&0x7)
		d.WindowSize = windowBase + windowAdd
	}

	// Read Dictionary_ID
	// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#dictionary_id
	d.DictionaryID = nil
	if size := fhd & 3; size != 0 {
		if size == 3 {
			size = 4
		}

		b, err := br.readSmall(int(size))
		if err != nil {
			println("Reading Dictionary_ID", err)
			return err
		}
		var id uint32
		switch size {
		case 1:
			id = uint32(b[0])
		case 2:
			id = uint32(b[0]) | (uint32(b[1]) << 8)
		case 4:
			id = uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24)
		}
		if debugDecoder {
			println("Dict size", size, "ID:", id)
		}
		if id > 0 {
			// ID 0 means "sorry, no dictionary anyway".
			// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#dictionary-format
			d.DictionaryID = &id
		}
	}

	// Read Frame_Content_Size
	// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#frame_content_size
	var fcsSize int
	v := fhd >> 6
	switch v {
	case 0:
		if d.SingleSegment {
			fcsSize = 1
		}
	default:
		fcsSize = 1 << v
	}
	d.FrameContentSize = fcsUnknown
	if fcsSize > 0 {
		b, err := br.readSmall(fcsSize)
		if err != nil {
			println("Reading Frame content", err)
			return err
		}
		switch fcsSize {
		case 1:
			d.FrameContentSize = uint64(b[0])
		case 2:
			// When FCS_Field_Size is 2, the offset of 256 is added.
			d.FrameContentSize = uint64(b[0]) | (uint64(b[1]) << 8) + 256
		case 4:
			d.FrameContentSize = uint64(b[0]) | (uint64(b[1]) << 8) | (uint64(b[2]) << 16) | (uint64(b[3]) << 24)
		case 8:
			d1 := uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24)
			d2 := uint32(b[4]) | (uint32(b[5]) << 8) | (uint32(b[6]) << 16) | (uint32(b[7]) << 24)
			d.FrameContentSize = uint64(d1) | (uint64(d2) << 32)
		}
		if debugDecoder {
			println("Read FCS:", d.FrameContentSize)
		}
	}

	// Move this to shared.
	d.HasCheckSum = fhd&(1<<2) != 0
	if d.HasCheckSum {
		if d.crc == nil {
			d.crc = xxhash.New()
		}
		d.crc.Reset()
	}

	if d.WindowSize == 0 && d.SingleSegment {
		// We may not need window in this case.
		d.WindowSize = d.FrameContentSize
		if d.WindowSize < MinWindowSize {
			d.WindowSize = MinWindowSize
		}
	}

	if d.WindowSize > uint64(d.o.maxWindowSize) {
		if debugDecoder {
			printf("window size %d > max %d\n", d.WindowSize, d.o.maxWindowSize)
		}
		return ErrWindowSizeExceeded
	}
	// The minimum Window_Size is 1 KB.
	if d.WindowSize < MinWindowSize {
		if debugDecoder {
			println("got window size: ", d.WindowSize)
		}
		return ErrWindowSizeTooSmall
	}
	d.history.windowSize = int(d.WindowSize)
	if !d.o.lowMem || d.history.windowSize < maxBlockSize {
		// Alloc 2x window size if not low-mem, or very small window size.
		d.history.allocFrameBuffer = d.history.windowSize * 2
	} else {
		// Alloc with one additional block
		d.history.allocFrameBuffer = d.history.windowSize + maxBlockSize
	}

	if debugDecoder {
		println("Frame: Dict:", d.DictionaryID, "FrameContentSize:", d.FrameContentSize, "singleseg:", d.SingleSegment, "window:", d.WindowSize, "crc:", d.HasCheckSum)
	}

	// history contains input - maybe we do something
	d.rawInput = br
	return nil
}

// next will start decoding the next block from stream.
func (d *frameDec) next(block *blockDec) error {
	if debugDecoder {
		println("decoding new block")
	}
	err := block.reset(d.rawInput, d.WindowSize)
	if err != nil {
		println("block error:", err)
		// Signal the frame decoder we have a problem.
		block.sendErr(err)
		return err
	}
	return nil
}

// checkCRC will check the checksum if the frame has one.
// Will return ErrCRCMismatch if crc check failed, otherwise nil.
func (d *frameDec) checkCRC() error {
	if !d.HasCheckSum {
		return nil
	}

	// We can overwrite upper tmp now
	want, err := d.rawInput.readSmall(4)
	if err != nil {
		println("CRC missing?", err)
		return err
	}

	if d.o.ignoreChecksum {
		return nil
	}

	var tmp [4]byte
	got := d.crc.Sum64()
	// Flip to match file order.
	tmp[0] = byte(got >> 0)
	tmp[1] = byte(got >> 8)
	tmp[2] = byte(got >> 16)
	tmp[3] = byte(got >> 24)

	if !bytes.Equal(tmp[:], want) {
		if debugDecoder {
			println("CRC Check Failed:", tmp[:], "!=", want)
		}
		return ErrCRCMismatch
	}
	if debugDecoder {
		println("CRC ok", tmp[:])
	}
	return nil
}

// consumeCRC reads the checksum data if the frame has one.
func (d *frameDec) consumeCRC() error {
	if d.HasCheckSum {
		_, err := d.rawInput.readSmall(4)
		if err != nil {
			println("CRC missing?", err)
			return err
		}
	}

	return nil
}

// runDecoder will create a sync decoder that will decode a block of data.
func (d *frameDec) runDecoder(dst []byte, dec *blockDec) ([]byte, error) {
	saved := d.history.b

	// We use the history for output to avoid copying it.
	d.history.b = dst
	d.history.ignoreBuffer = len(dst)
	// Store input length, so we only check new data.
	crcStart := len(dst)
	d.history.decoders.maxSyncLen = 0
	if d.FrameContentSize != fcsUnknown {
		d.history.decoders.maxSyncLen = d.FrameContentSize + uint64(len(dst))
		if d.history.decoders.maxSyncLen > d.o.maxDecodedSize {
			return dst, ErrDecoderSizeExceeded
		}
		if uint64(cap(dst)) < d.history.decoders.maxSyncLen {
			// Alloc for output
			dst2 := make([]byte, len(dst), d.history.decoders.maxSyncLen+compressedBlockOverAlloc)
			copy(dst2, dst)
			dst = dst2
		}
	}
	var err error
	for {
		err = dec.reset(d.rawInput, d.WindowSize)
		if err != nil {
			break
		}
		if debugDecoder {
			println("next block:", dec)
		}
		err = dec.decodeBuf(&d.history)
		if err != nil {
			break
		}
		if uint64(len(d.history.b)) > d.o.maxDecodedSize {
			err = ErrDecoderSizeExceeded
			break
		}
		if uint64(len(d.history.b)-crcStart) > d.FrameContentSize {
			println("runDecoder: FrameContentSize exceeded", uint64(len(d.history.b)-crcStart), ">", d.FrameContentSize)
			err = ErrFrameSizeExceeded
			break
		}
		if dec.Last {
			break
		}
		if debugDecoder {
			println("runDecoder: FrameContentSize", uint64(len(d.history.b)-crcStart), "<=", d.FrameContentSize)
		}
	}
	dst = d.history.b
	if err == nil {
		if d.FrameContentSize != fcsUnknown && uint64(len(d.history.b)-crcStart) != d.FrameContentSize {
			err = ErrFrameSizeMismatch
		} else if d.HasCheckSum {
			if d.o.ignoreChecksum {
				err = d.consumeCRC()
			} else {
				var n int
				n, err = d.crc.Write(dst[crcStart:])
				if err == nil {
					if n != len(dst)-crcStart {
						err = io.ErrShortWrite
					} else {
						err = d.checkCRC()
					}
				}
			}
		}
	}
	d.history.b = saved
	return dst, err
}
