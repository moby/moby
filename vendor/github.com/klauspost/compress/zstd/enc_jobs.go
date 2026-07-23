// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"fmt"
	rdebug "runtime/debug"
	"sync"
)

type encJob struct {
	prefix []byte        // overlap from previous job (nil for first)
	input  []byte        // job's own input data (swapped from filling)
	last   bool          // last block of last job gets last=true
	output []byte        // compressed blocks (filled by worker)
	err    error         // encoding error
	done   chan struct{} // closed when complete
}

type jobState struct {
	jobSize     int
	overlapSize int
	filling     []byte // accumulates input up to jobSize
	nextPrefix  []byte // overlap prefix prepared for the next dispatched job

	jobSeq int // next job sequence number

	jobCh    chan *encJob // dispatch to workers
	resultCh chan *encJob // ordered results to flusher

	workerWg  sync.WaitGroup
	flusherWg sync.WaitGroup

	mu         sync.Mutex
	flushedSeq int // last flushed sequence number
	cond       *sync.Cond

	flusherErr error
	started    bool

	inputPool   sync.Pool // *[]byte buffers of jobSize cap
	outputPool  sync.Pool // *[]byte buffers for compressed output
	overlapPool sync.Pool // *[]byte buffers for overlap prefixes
}

func (e *Encoder) startJobWorkers() {
	js := &e.state.jobs
	n := e.o.concurrent
	js.jobCh = make(chan *encJob, n)
	js.resultCh = make(chan *encJob, n)
	js.flushedSeq = 0
	js.cond = sync.NewCond(&js.mu)

	// Workers borrow encoders from the shared e.encoders pool per-job.
	// Ensure the pool is initialized before any worker tries to borrow.
	e.init.Do(e.initialize)

	for range n {
		js.workerWg.Add(1)
		go e.jobWorker()
	}
	js.flusherWg.Add(1)
	go e.jobFlusher()
	js.started = true
}

func (e *Encoder) jobWorker() {
	js := &e.state.jobs
	defer js.workerWg.Done()
	for job := range js.jobCh {
		enc := <-e.encoders
		e.compressJob(enc, job)
		e.encoders <- enc
		close(job.done)
	}
}

func (e *Encoder) compressJob(enc encoder, job *encJob) {
	defer func() {
		if r := recover(); r != nil {
			job.err = fmt.Errorf("panic in parallel job: %v", r)
			rdebug.PrintStack()
		}
	}()

	if len(job.prefix) > 0 {
		enc.ResetPrefix(job.prefix)
	} else {
		enc.Reset(nil, false)
	}

	data := job.input
	if len(data) == 0 && job.last {
		blk := enc.Block()
		blk.reset(nil)
		blk.last = true
		blk.encodeRaw(nil)
		job.output = append(job.output, blk.output...)
		return
	}

	blk := enc.Block()
	for len(data) > 0 {
		todo := data
		if len(todo) > e.o.blockSize {
			todo = todo[:e.o.blockSize]
		}
		data = data[len(todo):]

		blk.pushOffsets()
		enc.Encode(blk, todo)
		blk.last = len(data) == 0 && job.last

		err := blk.encode(todo, e.o.noEntropy, !e.o.allLitEntropy)
		if err != nil {
			job.err = err
			return
		}
		job.output = append(job.output, blk.output...)
		blk.reset(nil)
	}
}

func (js *jobState) getInputBuf(size int) []byte {
	if v := js.inputPool.Get(); v != nil {
		bp := v.(*[]byte)
		b := *bp
		if cap(b) >= size {
			return b[:0]
		}
	}
	return make([]byte, 0, size)
}

func (js *jobState) putInputBuf(b []byte) {
	if cap(b) > 0 {
		b = b[:0]
		js.inputPool.Put(&b)
	}
}

func (js *jobState) getOutputBuf(size int) []byte {
	if v := js.outputPool.Get(); v != nil {
		bp := v.(*[]byte)
		b := *bp
		if cap(b) >= size {
			return b[:0]
		}
	}
	return make([]byte, 0, size)
}

func (js *jobState) putOutputBuf(b []byte) {
	if cap(b) > 0 {
		b = b[:0]
		js.outputPool.Put(&b)
	}
}

func (js *jobState) getOverlapBuf(size int) []byte {
	if v := js.overlapPool.Get(); v != nil {
		bp := v.(*[]byte)
		b := *bp
		if cap(b) >= size {
			return b[:size]
		}
	}
	return make([]byte, size)
}

func (js *jobState) putOverlapBuf(b []byte) {
	if cap(b) > 0 {
		b = b[:0]
		js.overlapPool.Put(&b)
	}
}

func (e *Encoder) jobFlusher() {
	js := &e.state.jobs
	defer js.flusherWg.Done()
	for job := range js.resultCh {
		<-job.done
		// Worker has fully exited compressJob, so the prefix is no longer
		// in use. Return it to the pool regardless of outcome.
		if job.prefix != nil {
			js.putOverlapBuf(job.prefix)
			job.prefix = nil
		}
		if job.err != nil {
			js.mu.Lock()
			js.flusherErr = job.err
			js.cond.Broadcast()
			js.mu.Unlock()
			for range js.resultCh {
			}
			return
		}
		if len(job.output) > 0 {
			_, err := e.state.w.Write(job.output)
			if err != nil {
				js.mu.Lock()
				js.flusherErr = err
				js.cond.Broadcast()
				js.mu.Unlock()
				for range js.resultCh {
				}
				return
			}
			e.state.nWritten += int64(len(job.output))
		}
		// Return buffers to pools.
		js.putInputBuf(job.input)
		js.putOutputBuf(job.output)
		job.input = nil
		job.output = nil

		js.mu.Lock()
		js.flushedSeq++
		js.cond.Broadcast()
		js.mu.Unlock()
	}
}

func (e *Encoder) shutdownJobWorkers() {
	js := &e.state.jobs
	if !js.started {
		return
	}
	close(js.jobCh)
	js.workerWg.Wait()
	close(js.resultCh)
	js.flusherWg.Wait()
	js.started = false
}

// waitAllJobs blocks until all dispatched jobs have been flushed.
func (e *Encoder) waitAllJobs() {
	js := &e.state.jobs
	if !js.started {
		return
	}
	js.mu.Lock()
	for js.flushedSeq < js.jobSeq && js.flusherErr == nil {
		js.cond.Wait()
	}
	js.mu.Unlock()
}

func (e *Encoder) dispatchJob(final bool) error {
	s := &e.state
	js := &s.jobs

	js.mu.Lock()
	fErr := js.flusherErr
	js.mu.Unlock()
	if fErr != nil {
		return fErr
	}

	if !s.headerWritten {
		// Single-block optimization: fall through to encodeAll path.
		if final && len(js.filling) > 0 && len(js.filling) <= e.o.blockSize {
			s.current = e.encodeAll(s.encoder, js.filling, s.current[:0])
			var n2 int
			n2, s.err = s.w.Write(s.current)
			if s.err != nil {
				return s.err
			}
			s.nWritten += int64(n2)
			s.nInput += int64(len(js.filling))
			s.current = s.current[:0]
			js.filling = js.filling[:0]
			s.headerWritten = true
			s.fullFrameWritten = true
			s.eofWritten = true
			return nil
		}
		if final && len(js.filling) == 0 && !e.o.fullZero {
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
			DictID:        0,
		}
		dst := fh.appendTo(tmp[:0])
		var n2 int
		n2, s.err = s.w.Write(dst)
		if s.err != nil {
			return s.err
		}
		s.nWritten += int64(n2)
		s.headerWritten = true
	}

	if len(js.filling) == 0 && !final {
		return nil
	}

	if !js.started {
		e.startJobWorkers()
	}

	// Estimate output size for pooled buffer.
	outputEst := max(len(js.filling)/2, 512)

	job := &encJob{
		last:   final,
		done:   make(chan struct{}),
		output: js.getOutputBuf(outputEst),
	}

	// Each job owns its prefix slice; the flusher returns it to the pool
	// after <-job.done, so workers and dispatch never share a buffer.
	if js.nextPrefix != nil {
		job.prefix = js.nextPrefix
		js.nextPrefix = nil
	}

	// Build the next job's prefix from the tail of this job's input.
	if !final && len(js.filling) > 0 {
		overlapLen := min(js.overlapSize, len(js.filling))
		np := js.getOverlapBuf(overlapLen)
		copy(np, js.filling[len(js.filling)-overlapLen:])
		js.nextPrefix = np
	}

	// Swap filling buffer into job — zero-copy for the input data.
	job.input = js.filling
	js.filling = js.getInputBuf(js.jobSize)

	s.nInput += int64(len(job.input))
	js.jobSeq++

	if final {
		s.eofWritten = true
	}

	js.resultCh <- job
	js.jobCh <- job

	return nil
}
