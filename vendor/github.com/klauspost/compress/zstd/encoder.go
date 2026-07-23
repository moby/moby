// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	rdebug "runtime/debug"
	"sync"

	"github.com/klauspost/compress/zstd/internal/xxhash"
)

// Encoder provides encoding to Zstandard.
// An Encoder can be used for either compressing a stream via the
// io.WriteCloser interface supported by the Encoder or as multiple independent
// tasks via the EncodeAll function.
// Smaller encodes are encouraged to use the EncodeAll function.
// Use NewWriter to create a new instance.
type Encoder struct {
	o        encoderOptions
	encoders chan encoder
	state    encoderState
	init     sync.Once
}

type encoder interface {
	Encode(blk *blockEnc, src []byte)
	EncodeNoHist(blk *blockEnc, src []byte)
	Block() *blockEnc
	CRC() *xxhash.Digest
	AppendCRC([]byte) []byte
	WindowSize(size int64) int32
	UseBlock(*blockEnc)
	Reset(d *dict, singleBlock bool)
	ResetPrefix(prefix []byte)
}

type encoderState struct {
	w                io.Writer
	filling          []byte
	current          []byte
	previous         []byte
	encoder          encoder
	writing          *blockEnc
	err              error
	writeErr         error
	nWritten         int64
	nInput           int64
	frameContentSize int64
	headerWritten    bool
	eofWritten       bool
	fullFrameWritten bool

	// This waitgroup indicates an encode is running.
	wg sync.WaitGroup
	// This waitgroup indicates we have a block encoding/writing.
	wWg sync.WaitGroup

	// Parallel job state (used when concurrentBlocks is enabled).
	jobs jobState
}

// NewWriter will create a new Zstandard encoder.
// If the encoder will be used for encoding blocks a nil writer can be used.
func NewWriter(w io.Writer, opts ...EOption) (*Encoder, error) {
	initPredefined()
	var e Encoder
	e.o.setDefault()
	for _, o := range opts {
		err := o(&e.o)
		if err != nil {
			return nil, err
		}
	}
	if e.o.concurrentBlocks && (e.o.dict != nil || e.o.concurrent <= 1) {
		e.o.concurrentBlocks = false
	}
	if w != nil {
		e.Reset(w)
	}
	return &e, nil
}

func (e *Encoder) initialize() {
	if e.o.concurrent == 0 {
		e.o.setDefault()
	}
	e.encoders = make(chan encoder, e.o.concurrent)
	for i := 0; i < e.o.concurrent; i++ {
		enc := e.o.encoder()
		e.encoders <- enc
	}
}

// Reset will re-initialize the writer and new writes will encode to the supplied writer
// as a new, independent stream.
func (e *Encoder) Reset(w io.Writer) {
	s := &e.state

	if e.o.concurrentBlocks {
		e.shutdownJobWorkers()
		js := &s.jobs
		js.jobSize = e.o.jobSize()
		js.overlapSize = e.o.overlapSize()
		// js.filling is allocated lazily on first Write/ReadFrom so callers
		// that only use EncodeAll don't pay the (up to ~32 MB) jobSize cost.
		js.filling = js.filling[:0]
		if js.nextPrefix != nil {
			js.putOverlapBuf(js.nextPrefix)
			js.nextPrefix = nil
		}
		js.jobSeq = 0
		js.flushedSeq = 0
		js.flusherErr = nil
		js.started = false
	}

	s.wg.Wait()
	s.wWg.Wait()
	if cap(s.filling) == 0 {
		s.filling = make([]byte, 0, e.o.blockSize)
	}
	if e.o.concurrent > 1 && !e.o.concurrentBlocks {
		if cap(s.current) == 0 {
			s.current = make([]byte, 0, e.o.blockSize)
		}
		if cap(s.previous) == 0 {
			s.previous = make([]byte, 0, e.o.blockSize)
		}
		s.current = s.current[:0]
		s.previous = s.previous[:0]
		if s.writing == nil {
			s.writing = &blockEnc{lowMem: e.o.lowMem}
			s.writing.init()
		}
		s.writing.initNewEncode()
	}
	if s.encoder == nil {
		s.encoder = e.o.encoder()
	}
	s.filling = s.filling[:0]
	s.encoder.Reset(e.o.dict, false)
	s.headerWritten = false
	s.eofWritten = false
	s.fullFrameWritten = false
	s.w = w
	s.err = nil
	s.nWritten = 0
	s.nInput = 0
	s.writeErr = nil
	s.frameContentSize = 0
}

// ResetWithOptions will re-initialize the writer and apply the given options
// as a new, independent stream.
// Options are applied on top of the existing options.
// Some options cannot be changed on reset and will return an error.
func (e *Encoder) ResetWithOptions(w io.Writer, opts ...EOption) error {
	e.o.resetOpt = true
	defer func() { e.o.resetOpt = false }()
	hadDict := e.o.dict != nil
	for _, o := range opts {
		if err := o(&e.o); err != nil {
			return err
		}
	}
	hasDict := e.o.dict != nil
	if e.o.concurrentBlocks && hasDict {
		e.o.concurrentBlocks = false
	}
	if hadDict != hasDict {
		// Dict presence changed — encoder type must be recreated.
		e.state.encoder = nil
		e.init = sync.Once{}
	}
	e.Reset(w)
	return nil
}

// ResetContentSize will reset and set a content size for the next stream.
// If the bytes written does not match the size given an error will be returned
// when calling Close().
// This is removed when Reset is called.
// Sizes <= 0 results in no content size set.
func (e *Encoder) ResetContentSize(w io.Writer, size int64) {
	e.Reset(w)
	if size >= 0 {
		e.state.frameContentSize = size
	}
}

// Write data to the encoder.
// Input data will be buffered and as the buffer fills up
// content will be compressed and written to the output.
// When done writing, use Close to flush the remaining output
// and write CRC if requested.
func (e *Encoder) Write(p []byte) (n int, err error) {
	s := &e.state
	if s.eofWritten {
		return 0, ErrEncoderClosed
	}
	if e.o.concurrentBlocks {
		return e.writeJobs(p)
	}
	return e.writeBlocks(p)
}

func (e *Encoder) writeJobs(p []byte) (n int, err error) {
	s := &e.state
	js := &s.jobs
	jobSize := js.jobSize
	if cap(js.filling) == 0 && len(p) > 0 {
		js.filling = make([]byte, 0, jobSize)
	}
	for len(p) > 0 {
		if len(p)+len(js.filling) < jobSize {
			if e.o.crc {
				_, _ = s.encoder.CRC().Write(p)
			}
			js.filling = append(js.filling, p...)
			return n + len(p), nil
		}
		add := p
		if len(p)+len(js.filling) > jobSize {
			add = add[:jobSize-len(js.filling)]
		}
		if e.o.crc {
			_, _ = s.encoder.CRC().Write(add)
		}
		js.filling = append(js.filling, add...)
		p = p[len(add):]
		n += len(add)
		if len(js.filling) < jobSize {
			return n, nil
		}
		if err := e.dispatchJob(false); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (e *Encoder) writeBlocks(p []byte) (n int, err error) {
	s := &e.state
	for len(p) > 0 {
		if len(p)+len(s.filling) < e.o.blockSize {
			if e.o.crc {
				_, _ = s.encoder.CRC().Write(p)
			}
			s.filling = append(s.filling, p...)
			return n + len(p), nil
		}
		add := p
		if len(p)+len(s.filling) > e.o.blockSize {
			add = add[:e.o.blockSize-len(s.filling)]
		}
		if e.o.crc {
			_, _ = s.encoder.CRC().Write(add)
		}
		s.filling = append(s.filling, add...)
		p = p[len(add):]
		n += len(add)
		if len(s.filling) < e.o.blockSize {
			return n, nil
		}
		err := e.nextBlock(false)
		if err != nil {
			return n, err
		}
		if debugAsserts && len(s.filling) > 0 {
			panic(len(s.filling))
		}
	}
	return n, nil
}

// nextBlock will synchronize and start compressing input in e.state.filling.
// If an error has occurred during encoding it will be returned.
func (e *Encoder) nextBlock(final bool) error {
	s := &e.state
	// Wait for current block.
	s.wg.Wait()
	if s.err != nil {
		return s.err
	}
	if len(s.filling) > e.o.blockSize {
		return fmt.Errorf("block > maxStoreBlockSize")
	}
	if !s.headerWritten {
		// If we have a single block encode, do a sync compression.
		if final && len(s.filling) == 0 && !e.o.fullZero {
			s.headerWritten = true
			s.fullFrameWritten = true
			s.eofWritten = true
			return nil
		}
		if final && len(s.filling) > 0 {
			s.current = e.encodeAll(s.encoder, s.filling, s.current[:0])
			var n2 int
			n2, s.err = s.w.Write(s.current)
			if s.err != nil {
				return s.err
			}
			s.nWritten += int64(n2)
			s.nInput += int64(len(s.filling))
			s.current = s.current[:0]
			s.filling = s.filling[:0]
			s.headerWritten = true
			s.fullFrameWritten = true
			s.eofWritten = true
			return nil
		}

		var tmp [maxHeaderSize]byte
		fh := frameHeader{
			ContentSize:   uint64(s.frameContentSize),
			WindowSize:    uint32(s.encoder.WindowSize(s.frameContentSize)),
			SingleSegment: false,
			Checksum:      e.o.crc,
			DictID:        e.o.dict.ID(),
		}

		dst := fh.appendTo(tmp[:0])
		s.headerWritten = true
		s.wWg.Wait()
		var n2 int
		n2, s.err = s.w.Write(dst)
		if s.err != nil {
			return s.err
		}
		s.nWritten += int64(n2)
	}
	if s.eofWritten {
		// Ensure we only write it once.
		final = false
	}

	if len(s.filling) == 0 {
		// Final block, but no data.
		if final {
			enc := s.encoder
			blk := enc.Block()
			blk.reset(nil)
			blk.last = true
			blk.encodeRaw(nil)
			s.wWg.Wait()
			_, s.err = s.w.Write(blk.output)
			s.nWritten += int64(len(blk.output))
			s.eofWritten = true
		}
		return s.err
	}

	// SYNC:
	if e.o.concurrent == 1 {
		src := s.filling
		s.nInput += int64(len(s.filling))
		if debugEncoder {
			println("Adding sync block,", len(src), "bytes, final:", final)
		}
		enc := s.encoder
		blk := enc.Block()
		blk.reset(nil)
		enc.Encode(blk, src)
		blk.last = final
		if final {
			s.eofWritten = true
		}

		s.err = blk.encode(src, e.o.noEntropy, !e.o.allLitEntropy)
		if s.err != nil {
			return s.err
		}
		_, s.err = s.w.Write(blk.output)
		s.nWritten += int64(len(blk.output))
		s.filling = s.filling[:0]
		return s.err
	}

	// Move blocks forward.
	s.filling, s.current, s.previous = s.previous[:0], s.filling, s.current
	s.nInput += int64(len(s.current))
	s.wg.Add(1)
	if final {
		s.eofWritten = true
	}
	go func(src []byte) {
		if debugEncoder {
			println("Adding block,", len(src), "bytes, final:", final)
		}
		defer func() {
			if r := recover(); r != nil {
				s.err = fmt.Errorf("panic while encoding: %v", r)
				rdebug.PrintStack()
			}
			s.wg.Done()
		}()
		enc := s.encoder
		blk := enc.Block()
		enc.Encode(blk, src)
		blk.last = final
		// Wait for pending writes.
		s.wWg.Wait()
		if s.writeErr != nil {
			s.err = s.writeErr
			return
		}
		// Transfer encoders from previous write block.
		blk.swapEncoders(s.writing)
		// Transfer recent offsets to next.
		enc.UseBlock(s.writing)
		s.writing = blk
		s.wWg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.writeErr = fmt.Errorf("panic while encoding/writing: %v", r)
					rdebug.PrintStack()
				}
				s.wWg.Done()
			}()
			s.writeErr = blk.encode(src, e.o.noEntropy, !e.o.allLitEntropy)
			if s.writeErr != nil {
				return
			}
			_, s.writeErr = s.w.Write(blk.output)
			s.nWritten += int64(len(blk.output))
		}()
	}(s.current)
	return nil
}

// ReadFrom reads data from r until EOF or error.
// The return value n is the number of bytes read.
// Any error except io.EOF encountered during the read is also returned.
//
// The Copy function uses ReaderFrom if available.
func (e *Encoder) ReadFrom(r io.Reader) (n int64, err error) {
	if debugEncoder {
		println("Using ReadFrom")
	}

	if e.o.concurrentBlocks {
		return e.readFromJobs(r)
	}

	// Flush any current writes.
	if len(e.state.filling) > 0 {
		if err := e.nextBlock(false); err != nil {
			return 0, err
		}
	}
	e.state.filling = e.state.filling[:e.o.blockSize]
	src := e.state.filling
	for {
		n2, err := r.Read(src)
		if e.o.crc {
			_, _ = e.state.encoder.CRC().Write(src[:n2])
		}
		src = src[n2:]
		n += int64(n2)
		switch err {
		case io.EOF:
			e.state.filling = e.state.filling[:len(e.state.filling)-len(src)]
			if debugEncoder {
				println("ReadFrom: got EOF final block:", len(e.state.filling))
			}
			return n, nil
		case nil:
		default:
			if debugEncoder {
				println("ReadFrom: got error:", err)
			}
			e.state.err = err
			return n, err
		}
		if len(src) > 0 {
			if debugEncoder {
				println("ReadFrom: got space left in source:", len(src))
			}
			continue
		}
		err = e.nextBlock(false)
		if err != nil {
			return n, err
		}
		e.state.filling = e.state.filling[:e.o.blockSize]
		src = e.state.filling
	}
}

func (e *Encoder) readFromJobs(r io.Reader) (n int64, err error) {
	js := &e.state.jobs
	jobSize := js.jobSize

	// Flush any current filling.
	if len(js.filling) > 0 {
		if err := e.dispatchJob(false); err != nil {
			return 0, err
		}
	}

	if cap(js.filling) < jobSize {
		js.filling = make([]byte, 0, jobSize)
	}
	js.filling = js.filling[:jobSize]
	src := js.filling
	for {
		n2, err := r.Read(src)
		if e.o.crc {
			_, _ = e.state.encoder.CRC().Write(src[:n2])
		}
		src = src[n2:]
		n += int64(n2)
		switch err {
		case io.EOF:
			js.filling = js.filling[:len(js.filling)-len(src)]
			return n, nil
		case nil:
		default:
			e.state.err = err
			return n, err
		}
		if len(src) > 0 {
			continue
		}
		if err = e.dispatchJob(false); err != nil {
			return n, err
		}
		if cap(js.filling) < jobSize {
			js.filling = make([]byte, 0, jobSize)
		}
		js.filling = js.filling[:jobSize]
		src = js.filling
	}
}

// Flush will send the currently written data to output
// and block until everything has been written.
// This should only be used on rare occasions where pushing the currently queued data is critical.
func (e *Encoder) Flush() error {
	s := &e.state
	if e.o.concurrentBlocks {
		return e.flushJobs()
	}
	if len(s.filling) > 0 {
		err := e.nextBlock(false)
		if err != nil {
			if errors.Is(s.err, ErrEncoderClosed) {
				return nil
			}
			return err
		}
	}
	s.wg.Wait()
	s.wWg.Wait()
	if s.err != nil {
		if errors.Is(s.err, ErrEncoderClosed) {
			return nil
		}
		return s.err
	}
	return s.writeErr
}

func (e *Encoder) flushJobs() error {
	js := &e.state.jobs
	if len(js.filling) > 0 {
		if err := e.dispatchJob(false); err != nil {
			return err
		}
	}
	e.waitAllJobs()
	js.mu.Lock()
	fErr := js.flusherErr
	js.mu.Unlock()
	return fErr
}

// Close will flush the final output and close the stream.
// The function will block until everything has been written.
// The Encoder can still be re-used after calling this.
func (e *Encoder) Close() error {
	s := &e.state
	if s.encoder == nil {
		return nil
	}
	if e.o.concurrentBlocks {
		return e.closeJobs()
	}
	if s.w == nil {
		if len(s.filling) == 0 && !s.headerWritten && !s.eofWritten && s.nInput == 0 {
			return nil
		}
		return errors.New("zstd: encoder has no writer")
	}

	err := e.nextBlock(true)
	if err != nil {
		if errors.Is(s.err, ErrEncoderClosed) {
			return nil
		}
		return err
	}
	if s.frameContentSize > 0 {
		if s.nInput != s.frameContentSize {
			return fmt.Errorf("frame content size %d given, but %d bytes was written", s.frameContentSize, s.nInput)
		}
	}
	if e.state.fullFrameWritten {
		return s.err
	}
	s.wg.Wait()
	s.wWg.Wait()

	if s.err != nil {
		return s.err
	}
	if s.writeErr != nil {
		return s.writeErr
	}

	// Write CRC
	if e.o.crc && s.err == nil {
		// heap alloc.
		var tmp [4]byte
		_, s.err = s.w.Write(s.encoder.AppendCRC(tmp[:0]))
		s.nWritten += 4
	}

	// Add padding with content from crypto/rand.Reader
	if s.err == nil && e.o.pad > 0 {
		add := calcSkippableFrame(s.nWritten, int64(e.o.pad))
		frame, err := skippableFrame(s.filling[:0], add, rand.Reader)
		if err != nil {
			return err
		}
		_, s.err = s.w.Write(frame)
	}
	if s.err == nil {
		s.err = ErrEncoderClosed
		return nil
	}

	return s.err
}

func (e *Encoder) closeJobs() error {
	s := &e.state
	js := &s.jobs

	if errors.Is(s.err, ErrEncoderClosed) {
		return nil
	}

	if s.w == nil {
		if len(js.filling) == 0 && !s.headerWritten && !s.eofWritten && s.nInput == 0 {
			return nil
		}
		return errors.New("zstd: encoder has no writer")
	}

	if err := e.dispatchJob(true); err != nil {
		e.shutdownJobWorkers()
		if errors.Is(s.err, ErrEncoderClosed) {
			return nil
		}
		return err
	}

	if s.frameContentSize > 0 && s.nInput != s.frameContentSize {
		e.shutdownJobWorkers()
		return fmt.Errorf("frame content size %d given, but %d bytes was written", s.frameContentSize, s.nInput)
	}

	if s.fullFrameWritten {
		e.shutdownJobWorkers()
		s.err = ErrEncoderClosed
		return nil
	}

	e.shutdownJobWorkers()
	if js.flusherErr != nil {
		return js.flusherErr
	}

	// Write CRC
	if e.o.crc {
		var tmp [4]byte
		_, s.err = s.w.Write(s.encoder.AppendCRC(tmp[:0]))
		s.nWritten += 4
	}

	// Add padding
	if s.err == nil && e.o.pad > 0 {
		add := calcSkippableFrame(s.nWritten, int64(e.o.pad))
		frame, err := skippableFrame(js.filling[:0], add, rand.Reader)
		if err != nil {
			return err
		}
		_, s.err = s.w.Write(frame)
	}
	if s.err == nil {
		s.err = ErrEncoderClosed
		return nil
	}
	return s.err
}

// EncodeAll will encode all input in src and append it to dst.
// This function can be called concurrently, but each call will only run on a single goroutine.
// If empty input is given, nothing is returned, unless WithZeroFrames is specified.
// Encoded blocks can be concatenated and the result will be the combined input stream.
// Data compressed with EncodeAll can be decoded with the Decoder,
// using either a stream or DecodeAll.
func (e *Encoder) EncodeAll(src, dst []byte) []byte {
	e.init.Do(e.initialize)
	enc := <-e.encoders
	defer func() {
		e.encoders <- enc
	}()
	return e.encodeAll(enc, src, dst)
}

func (e *Encoder) encodeAll(enc encoder, src, dst []byte) []byte {
	if len(src) == 0 {
		if e.o.fullZero {
			// Add frame header.
			fh := frameHeader{
				ContentSize:   0,
				WindowSize:    MinWindowSize,
				SingleSegment: true,
				// Adding a checksum would be a waste of space.
				Checksum: false,
				DictID:   0,
			}
			dst = fh.appendTo(dst)

			// Write raw block as last one only.
			var blk blockHeader
			blk.setSize(0)
			blk.setType(blockTypeRaw)
			blk.setLast(true)
			dst = blk.appendTo(dst)
		}
		return dst
	}

	// Use single segments when above minimum window and below window size.
	single := len(src) <= e.o.windowSize && len(src) > MinWindowSize
	if e.o.single != nil {
		single = *e.o.single
	}
	fh := frameHeader{
		ContentSize:   uint64(len(src)),
		WindowSize:    uint32(enc.WindowSize(int64(len(src)))),
		SingleSegment: single,
		Checksum:      e.o.crc,
		DictID:        e.o.dict.ID(),
	}

	// If less than 1MB, allocate a buffer up front.
	if len(dst) == 0 && cap(dst) == 0 && len(src) < 1<<20 && !e.o.lowMem {
		dst = make([]byte, 0, len(src))
	}
	dst = fh.appendTo(dst)

	// If we can do everything in one block, prefer that.
	if len(src) <= e.o.blockSize {
		enc.Reset(e.o.dict, true)
		// Slightly faster with no history and everything in one block.
		if e.o.crc {
			_, _ = enc.CRC().Write(src)
		}
		blk := enc.Block()
		blk.last = true
		if e.o.dict == nil {
			enc.EncodeNoHist(blk, src)
		} else {
			enc.Encode(blk, src)
		}

		// If we got the exact same number of literals as input,
		// assume the literals cannot be compressed.
		oldout := blk.output
		// Output directly to dst
		blk.output = dst

		err := blk.encode(src, e.o.noEntropy, !e.o.allLitEntropy)
		if err != nil {
			panic(err)
		}
		dst = blk.output
		blk.output = oldout
	} else {
		enc.Reset(e.o.dict, false)
		blk := enc.Block()
		for len(src) > 0 {
			todo := src
			if len(todo) > e.o.blockSize {
				todo = todo[:e.o.blockSize]
			}
			src = src[len(todo):]
			if e.o.crc {
				_, _ = enc.CRC().Write(todo)
			}
			blk.pushOffsets()
			enc.Encode(blk, todo)
			if len(src) == 0 {
				blk.last = true
			}
			err := blk.encode(todo, e.o.noEntropy, !e.o.allLitEntropy)
			if err != nil {
				panic(err)
			}
			dst = append(dst, blk.output...)
			blk.reset(nil)
		}
	}
	if e.o.crc {
		dst = enc.AppendCRC(dst)
	}
	// Add padding with content from crypto/rand.Reader
	if e.o.pad > 0 {
		add := calcSkippableFrame(int64(len(dst)), int64(e.o.pad))
		var err error
		dst, err = skippableFrame(dst, add, rand.Reader)
		if err != nil {
			panic(err)
		}
	}
	return dst
}

// MaxEncodedSize returns the expected maximum
// size of an encoded block or stream.
func (e *Encoder) MaxEncodedSize(size int) int {
	frameHeader := 4 + 2 // magic + frame header & window descriptor
	if e.o.dict != nil {
		frameHeader += 4
	}
	// Frame content size:
	if size < 256 {
		frameHeader++
	} else if size < 65536+256 {
		frameHeader += 2
	} else if size < math.MaxInt32 {
		frameHeader += 4
	} else {
		frameHeader += 8
	}
	// Final crc
	if e.o.crc {
		frameHeader += 4
	}

	// Max overhead is 3 bytes/block.
	// There cannot be 0 blocks.
	blocks := (size + e.o.blockSize) / e.o.blockSize

	// Combine, add padding.
	maxSz := frameHeader + 3*blocks + size
	if e.o.pad > 1 {
		maxSz += calcSkippableFrame(int64(maxSz), int64(e.o.pad))
	}
	return maxSz
}
