// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pgzip

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/crc32"
)

const (
	defaultBlockSize = 256 << 10
	tailSize         = 16384
	defaultBlocks    = 16
)

// These constants are copied from the flate package, so that code that imports
// "compress/gzip" does not also have to import "compress/flate".
const (
	NoCompression       = flate.NoCompression
	BestSpeed           = flate.BestSpeed
	BestCompression     = flate.BestCompression
	DefaultCompression  = flate.DefaultCompression
	ConstantCompression = flate.ConstantCompression
	HuffmanOnly         = flate.HuffmanOnly
)

// A Writer is an io.WriteCloser.
// Writes to a Writer are compressed and written to w.
type Writer struct {
	Header
	w             io.Writer
	level         int
	wroteHeader   bool
	blockSize     int
	blocks        int
	currentBuffer []byte
	prevTail      []byte
	digest        hash.Hash32
	size          int
	closed        bool
	buf           [10]byte
	errMu         sync.RWMutex
	err           error
	pushedErr     chan struct{}
	results       chan result
	dictFlatePool sync.Pool
	dstPool       sync.Pool
	wg            sync.WaitGroup
}

type result struct {
	result        chan []byte
	notifyWritten chan struct{}
}

// Use SetConcurrency to finetune the concurrency level if needed.
//
// With this you can control the approximate size of your blocks,
// as well as how many you want to be processing in parallel.
//
// Default values for this is SetConcurrency(250000, 16),
// meaning blocks are split at 250000 bytes and up to 16 blocks
// can be processing at once before the writer blocks.
func (z *Writer) SetConcurrency(blockSize, blocks int) error {
	if blockSize <= tailSize {
		return fmt.Errorf("gzip: block size cannot be less than or equal to %d", tailSize)
	}
	if blocks <= 0 {
		return errors.New("gzip: blocks cannot be zero or less")
	}
	if blockSize == z.blockSize && blocks == z.blocks {
		return nil
	}
	z.blockSize = blockSize
	z.results = make(chan result, blocks)
	z.blocks = blocks
	z.dstPool = sync.Pool{New: func() interface{} { return make([]byte, 0, blockSize+(blockSize)>>4) }}
	return nil
}

// NewWriter returns a new Writer.
// Writes to the returned writer are compressed and written to w.
//
// It is the caller's responsibility to call Close on the WriteCloser when done.
// Writes may be buffered and not flushed until Close.
//
// Callers that wish to set the fields in Writer.Header must do so before
// the first call to Write or Close. The Comment and Name header fields are
// UTF-8 strings in Go, but the underlying format requires NUL-terminated ISO
// 8859-1 (Latin-1). NUL or non-Latin-1 runes in those strings will lead to an
// error on Write.
func NewWriter(w io.Writer) *Writer {
	z, _ := NewWriterLevel(w, DefaultCompression)
	return z
}

// NewWriterLevel is like NewWriter but specifies the compression level instead
// of assuming DefaultCompression.
//
// The compression level can be DefaultCompression, NoCompression, or any
// integer value between BestSpeed and BestCompression inclusive. The error
// returned will be nil if the level is valid.
func NewWriterLevel(w io.Writer, level int) (*Writer, error) {
	if level < ConstantCompression || level > BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", level)
	}
	z := new(Writer)
	z.SetConcurrency(defaultBlockSize, defaultBlocks)
	z.init(w, level)
	return z, nil
}

// This function must be used by goroutines to set an
// error condition, since z.err access is restricted
// to the callers goruotine.
func (z *Writer) pushError(err error) {
	z.errMu.Lock()
	if z.err != nil {
		z.errMu.Unlock()
		return
	}
	z.err = err
	close(z.pushedErr)
	z.errMu.Unlock()
}

func (z *Writer) init(w io.Writer, level int) {
	z.wg.Wait()
	digest := z.digest
	if digest != nil {
		digest.Reset()
	} else {
		digest = crc32.NewIEEE()
	}
	z.Header = Header{OS: 255}
	z.w = w
	z.level = level
	z.digest = digest
	z.pushedErr = make(chan struct{}, 0)
	z.results = make(chan result, z.blocks)
	z.err = nil
	z.closed = false
	z.Comment = ""
	z.Extra = nil
	z.ModTime = time.Time{}
	z.wroteHeader = false
	z.currentBuffer = nil
	z.buf = [10]byte{}
	z.prevTail = nil
	z.size = 0
	if z.dictFlatePool.New == nil {
		z.dictFlatePool.New = func() interface{} {
			f, _ := flate.NewWriterDict(w, level, nil)
			return f
		}
	}
}

// Reset discards the Writer z's state and makes it equivalent to the
// result of its original state from NewWriter or NewWriterLevel, but
// writing to w instead. This permits reusing a Writer rather than
// allocating a new one.
func (z *Writer) Reset(w io.Writer) {
	if z.results != nil && !z.closed {
		close(z.results)
	}
	z.SetConcurrency(defaultBlockSize, defaultBlocks)
	z.init(w, z.level)
}

// GZIP (RFC 1952) is little-endian, unlike ZLIB (RFC 1950).
func put2(p []byte, v uint16) {
	p[0] = uint8(v >> 0)
	p[1] = uint8(v >> 8)
}

func put4(p []byte, v uint32) {
	p[0] = uint8(v >> 0)
	p[1] = uint8(v >> 8)
	p[2] = uint8(v >> 16)
	p[3] = uint8(v >> 24)
}

// writeBytes writes a length-prefixed byte slice to z.w.
func (z *Writer) writeBytes(b []byte) error {
	if len(b) > 0xffff {
		return errors.New("gzip.Write: Extra data is too large")
	}
	put2(z.buf[0:2], uint16(len(b)))
	_, err := z.w.Write(z.buf[0:2])
	if err != nil {
		return err
	}
	_, err = z.w.Write(b)
	return err
}

// writeString writes a UTF-8 string s in GZIP's format to z.w.
// GZIP (RFC 1952) specifies that strings are NUL-terminated ISO 8859-1 (Latin-1).
func (z *Writer) writeString(s string) (err error) {
	// GZIP stores Latin-1 strings; error if non-Latin-1; convert if non-ASCII.
	needconv := false
	for _, v := range s {
		if v == 0 || v > 0xff {
			return errors.New("gzip.Write: non-Latin-1 header string")
		}
		if v > 0x7f {
			needconv = true
		}
	}
	if needconv {
		b := make([]byte, 0, len(s))
		for _, v := range s {
			b = append(b, byte(v))
		}
		_, err = z.w.Write(b)
	} else {
		_, err = io.WriteString(z.w, s)
	}
	if err != nil {
		return err
	}
	// GZIP strings are NUL-terminated.
	z.buf[0] = 0
	_, err = z.w.Write(z.buf[0:1])
	return err
}

// compressCurrent will compress the data currently buffered
// This should only be called from the main writer/flush/closer
func (z *Writer) compressCurrent(flush bool) {
	r := result{}
	r.result = make(chan []byte, 1)
	r.notifyWritten = make(chan struct{}, 0)
	select {
	case z.results <- r:
	case <-z.pushedErr:
		return
	}

	// If block given is more than twice the block size, split it.
	c := z.currentBuffer
	if len(c) > z.blockSize*2 {
		c = c[:z.blockSize]
		z.wg.Add(1)
		go z.compressBlock(c, z.prevTail, r, false)
		z.prevTail = c[len(c)-tailSize:]
		z.currentBuffer = z.currentBuffer[z.blockSize:]
		z.compressCurrent(flush)
		// Last one flushes if needed
		return
	}

	z.wg.Add(1)
	go z.compressBlock(c, z.prevTail, r, z.closed)
	if len(c) > tailSize {
		z.prevTail = c[len(c)-tailSize:]
	} else {
		z.prevTail = nil
	}
	z.currentBuffer = z.dstPool.Get().([]byte)
	z.currentBuffer = z.currentBuffer[:0]

	// Wait if flushing
	if flush {
		<-r.notifyWritten
	}
}

// Returns an error if it has been set.
// Cannot be used by functions that are from internal goroutines.
func (z *Writer) checkError() error {
	z.errMu.RLock()
	err := z.err
	z.errMu.RUnlock()
	return err
}

// Write writes a compressed form of p to the underlying io.Writer. The
// compressed bytes are not necessarily flushed to output until
// the Writer is closed or Flush() is called.
//
// The function will return quickly, if there are unused buffers.
// The sent slice (p) is copied, and the caller is free to re-use the buffer
// when the function returns.
//
// Errors that occur during compression will be reported later, and a nil error
// does not signify that the compression succeeded (since it is most likely still running)
// That means that the call that returns an error may not be the call that caused it.
// Only Flush and Close functions are guaranteed to return any errors up to that point.
func (z *Writer) Write(p []byte) (int, error) {
	if err := z.checkError(); err != nil {
		return 0, err
	}
	// Write the GZIP header lazily.
	if !z.wroteHeader {
		z.wroteHeader = true
		z.buf[0] = gzipID1
		z.buf[1] = gzipID2
		z.buf[2] = gzipDeflate
		z.buf[3] = 0
		if z.Extra != nil {
			z.buf[3] |= 0x04
		}
		if z.Name != "" {
			z.buf[3] |= 0x08
		}
		if z.Comment != "" {
			z.buf[3] |= 0x10
		}
		put4(z.buf[4:8], uint32(z.ModTime.Unix()))
		if z.level == BestCompression {
			z.buf[8] = 2
		} else if z.level == BestSpeed {
			z.buf[8] = 4
		} else {
			z.buf[8] = 0
		}
		z.buf[9] = z.OS
		var n int
		var err error
		n, err = z.w.Write(z.buf[0:10])
		if err != nil {
			z.pushError(err)
			return n, err
		}
		if z.Extra != nil {
			err = z.writeBytes(z.Extra)
			if err != nil {
				z.pushError(err)
				return n, err
			}
		}
		if z.Name != "" {
			err = z.writeString(z.Name)
			if err != nil {
				z.pushError(err)
				return n, err
			}
		}
		if z.Comment != "" {
			err = z.writeString(z.Comment)
			if err != nil {
				z.pushError(err)
				return n, err
			}
		}
		// Start receiving data from compressors
		go func() {
			listen := z.results
			for {
				r, ok := <-listen
				// If closed, we are finished.
				if !ok {
					return
				}
				buf := <-r.result
				n, err := z.w.Write(buf)
				if err != nil {
					z.pushError(err)
					close(r.notifyWritten)
					return
				}
				if n != len(buf) {
					z.pushError(fmt.Errorf("gzip: short write %d should be %d", n, len(buf)))
					close(r.notifyWritten)
					return
				}
				z.dstPool.Put(buf)
				close(r.notifyWritten)
			}
		}()
		z.currentBuffer = make([]byte, 0, z.blockSize)
	}
	q := p
	for len(q) > 0 {
		length := len(q)
		if length+len(z.currentBuffer) > z.blockSize {
			length = z.blockSize - len(z.currentBuffer)
		}
		z.digest.Write(q[:length])
		z.currentBuffer = append(z.currentBuffer, q[:length]...)
		if len(z.currentBuffer) >= z.blockSize {
			z.compressCurrent(false)
			if err := z.checkError(); err != nil {
				return len(p) - len(q) - length, err
			}
		}
		z.size += length
		q = q[length:]
	}
	return len(p), z.checkError()
}

// Step 1: compresses buffer to buffer
// Step 2: send writer to channel
// Step 3: Close result channel to indicate we are done
func (z *Writer) compressBlock(p, prevTail []byte, r result, closed bool) {
	defer func() {
		close(r.result)
		z.wg.Done()
	}()
	buf := z.dstPool.Get().([]byte)
	dest := bytes.NewBuffer(buf[:0])

	compressor := z.dictFlatePool.Get().(*flate.Writer)
	compressor.ResetDict(dest, prevTail)
	compressor.Write(p)

	err := compressor.Flush()
	if err != nil {
		z.pushError(err)
		return
	}
	if closed {
		err = compressor.Close()
		if err != nil {
			z.pushError(err)
			return
		}
	}
	z.dictFlatePool.Put(compressor)
	// Read back buffer
	buf = dest.Bytes()
	r.result <- buf
}

// Flush flushes any pending compressed data to the underlying writer.
//
// It is useful mainly in compressed network protocols, to ensure that
// a remote reader has enough data to reconstruct a packet. Flush does
// not return until the data has been written. If the underlying
// writer returns an error, Flush returns that error.
//
// In the terminology of the zlib library, Flush is equivalent to Z_SYNC_FLUSH.
func (z *Writer) Flush() error {
	if err := z.checkError(); err != nil {
		return err
	}
	if z.closed {
		return nil
	}
	if !z.wroteHeader {
		_, err := z.Write(nil)
		if err != nil {
			return err
		}
	}
	// We send current block to compression
	z.compressCurrent(true)

	return z.checkError()
}

// UncompressedSize will return the number of bytes written.
// pgzip only, not a function in the official gzip package.
func (z *Writer) UncompressedSize() int {
	return z.size
}

// Close closes the Writer, flushing any unwritten data to the underlying
// io.Writer, but does not close the underlying io.Writer.
func (z *Writer) Close() error {
	if err := z.checkError(); err != nil {
		return err
	}
	if z.closed {
		return nil
	}

	z.closed = true
	if !z.wroteHeader {
		z.Write(nil)
		if err := z.checkError(); err != nil {
			return err
		}
	}
	z.compressCurrent(true)
	if err := z.checkError(); err != nil {
		return err
	}
	close(z.results)
	put4(z.buf[0:4], z.digest.Sum32())
	put4(z.buf[4:8], uint32(z.size))
	_, err := z.w.Write(z.buf[0:8])
	if err != nil {
		z.pushError(err)
		return err
	}
	return nil
}
