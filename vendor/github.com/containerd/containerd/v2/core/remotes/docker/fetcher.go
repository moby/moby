/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package docker

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
)

type bufferPool struct {
	pool *sync.Pool
}

func newbufferPool(bufCap int64) *bufferPool {
	pool := &sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, bufCap))
		},
	}
	return &bufferPool{
		pool: pool,
	}
}

func (p *bufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get().(*bytes.Buffer)
	return buf
}

func (p *bufferPool) Put(buffer *bytes.Buffer) {
	p.pool.Put(buffer)
}

var ErrClosedPipe = errors.New("bufpipe: read/write on closed pipe")

// pipe implements an asynchronous buffered pipe designed for high-throughput
// I/O with configurable initial buffer sizes and buffer reuse. It decouples
// read/write operations, allowing writers to proceed without blocking (unless
// the pipe is closed) and readers to wait efficiently for incoming data.
//
// Key Characteristics:
//   - Asynchronous Operation: Writers populate buffers independently of read
//     timing, enabling continuous data flow without reader-writer synchronization.
//   - Dynamic Buffering: Active buffer grows organically to handle large payloads, while
//     the initial capacity (bufCap) balances memory pre-allocation and growth overhead.
//   - Buffer Recycling: Retrieved from/pushed to a pool to minimize allocations, reducing
//     garbage collection pressure in sustained I/O scenarios.
//   - Error Semantics: Closes deterministically on first error (read or write), propagating
//     errors atomically to both ends while draining buffered data.
//
// Difference with io.Pipe:
//   - Unlike io.Pipe's strict synchronization (blocking write until read), this implementation
//     allows writers to buffer data ahead of reads, improving throughput for bursty workloads.
//
// Synchronization & Internals:
//   - Condition Variable (sync.Cond): Coordinates reader/writer, waking readers on new data
//     or closure. Locking is centralized via the condition's mutex.
//   - Buffer Lifecycle: Active buffer serves writes until read depletion, after which it's
//     recycled to the pool. Pooled buffers retain their capacity across uses.
//   - Error Handling: Write errors (werr) permanently fail writes; read errors (rerr) mark
//     terminal read state after buffer exhaustion.
//
// Future Considerations:
// - Zero-copy reads/writes to avoid buffer copying overhead.
// - Memory-mapped file backing for multi-gigabyte payloads.
type pipe struct {
	cond       *sync.Cond    // Coordinates read/write signaling via Lock+Wait/Signal
	bufPool    *bufferPool   // Reusable buffers with initial capacity bufCap
	buf        *bytes.Buffer // Active data buffer (nil when empty/returned to pool)
	rerr, werr error         // Terminal read/write errors (sticky once set)
}

type pipeReader struct {
	*pipe
}

type pipeWriter struct {
	*pipe
}

func newPipeWriter(bufPool *bufferPool) (*pipeReader, *pipeWriter) {
	p := &pipe{
		cond:    sync.NewCond(new(sync.Mutex)),
		bufPool: bufPool,
		buf:     nil,
	}
	return &pipeReader{
			pipe: p,
		}, &pipeWriter{
			pipe: p,
		}
}

// Read implements the standard Read interface: it reads data from the pipe,
// reading from the internal buffer, otherwise blocking until a writer arrives
// or the write end is closed. If the write end is closed with an error, that
// error is returned as err; otherwise err is io.EOF.
func (r *pipeReader) Read(data []byte) (n int, err error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if r.buf == nil {
		r.buf = r.bufPool.Get()
	}

	for {
		n, err = r.buf.Read(data)
		// If not closed and no read, wait for writing.
		if err == io.EOF && r.rerr == nil && n == 0 {
			r.cond.Wait() // Wait for data to be written
			continue
		}
		break
	}
	if err == io.EOF {
		// Put buffer back to pool
		r.bufPool.Put(r.buf)
		r.buf = nil
		return n, r.rerr
	}
	return n, err
}

// Close closes the reader; subsequent writes from the write half of the pipe
// will return error ErrClosedPipe.
func (r *pipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes to the write half of the
// pipe will return the error err.
func (r *pipeReader) CloseWithError(err error) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if err == nil {
		err = ErrClosedPipe
	}
	r.werr = err
	return nil
}

// Write implements the standard Write interface: it writes data to the internal
// buffer. If the read end is closed with an error, that err is returned as err;
// otherwise err is ErrClosedPipe.
func (w *pipeWriter) Write(data []byte) (int, error) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	if w.werr != nil {
		return 0, w.werr
	}

	if w.buf == nil {
		w.buf = w.bufPool.Get()
	}

	n, err := w.buf.Write(data)
	w.cond.Signal()
	return n, err
}

// Close closes the writer; subsequent reads from the read half of the pipe will
// return io.EOF once the internal buffer get empty.
func (w *pipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// Close closes the writer; subsequent reads from the read half of the pipe will
// return err once the internal buffer get empty.
func (w *pipeWriter) CloseWithError(err error) error {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	if err == nil {
		err = io.EOF
	}
	w.rerr = err
	w.cond.Broadcast()
	return nil
}

type dockerFetcher struct {
	*dockerBase
}

func (r dockerFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", desc.Digest))

	hosts := r.filterHosts(HostCapabilityPull)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no pull hosts: %w", errdefs.ErrNotFound)
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, err
	}

	return newHTTPReadSeeker(desc.Size, func(offset int64) (io.ReadCloser, error) {
		// firstly try fetch via external urls
		for _, us := range desc.URLs {
			u, err := url.Parse(us)
			if err != nil {
				log.G(ctx).WithError(err).Debugf("failed to parse %q", us)
				continue
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				log.G(ctx).Debug("non-http(s) alternative url is unsupported")
				continue
			}
			ctx = log.WithLogger(ctx, log.G(ctx).WithField("url", u))
			log.G(ctx).Info("request")

			// Try this first, parse it
			host := RegistryHost{
				Client:       http.DefaultClient,
				Host:         u.Host,
				Scheme:       u.Scheme,
				Path:         u.Path,
				Capabilities: HostCapabilityPull,
			}
			req := r.request(host, http.MethodGet)
			// Strip namespace from base
			req.path = u.Path
			if u.RawQuery != "" {
				req.path = req.path + "?" + u.RawQuery
			}

			rc, err := r.open(ctx, req, desc.MediaType, offset, false)
			if err != nil {
				if errdefs.IsNotFound(err) {
					continue // try one of the other urls.
				}

				return nil, err
			}

			return rc, nil
		}

		// Try manifests endpoints for manifests types
		if images.IsManifestType(desc.MediaType) || images.IsIndexType(desc.MediaType) {

			var firstErr error
			for i, host := range r.hosts {
				req := r.request(host, http.MethodGet, "manifests", desc.Digest.String())
				if err := req.addNamespace(r.refspec.Hostname()); err != nil {
					return nil, err
				}

				rc, err := r.open(ctx, req, desc.MediaType, offset, i == len(r.hosts)-1)
				if err != nil {
					// Store the error for referencing later
					if firstErr == nil {
						firstErr = err
					}
					continue // try another host
				}

				return rc, nil
			}

			return nil, firstErr
		}

		// Finally use blobs endpoints
		var firstErr error
		for i, host := range r.hosts {
			req := r.request(host, http.MethodGet, "blobs", desc.Digest.String())
			if err := req.addNamespace(r.refspec.Hostname()); err != nil {
				return nil, err
			}

			rc, err := r.open(ctx, req, desc.MediaType, offset, i == len(r.hosts)-1)
			if err != nil {
				// Store the error for referencing later
				if firstErr == nil {
					firstErr = err
				}
				continue // try another host
			}

			return rc, nil
		}

		if errdefs.IsNotFound(firstErr) {
			firstErr = fmt.Errorf("could not fetch content descriptor %v (%v) from remote: %w",
				desc.Digest, desc.MediaType, errdefs.ErrNotFound,
			)
		}

		return nil, firstErr

	})
}

func (r dockerFetcher) createGetReq(ctx context.Context, host RegistryHost, lastHost bool, mediatype string, ps ...string) (*request, int64, error) {
	headReq := r.request(host, http.MethodHead, ps...)
	if err := headReq.addNamespace(r.refspec.Hostname()); err != nil {
		return nil, 0, err
	}

	if mediatype == "" {
		headReq.header.Set("Accept", "*/*")
	} else {
		headReq.header.Set("Accept", strings.Join([]string{mediatype, `*/*`}, ", "))
	}

	headResp, err := headReq.doWithRetries(ctx, lastHost)
	if err != nil {
		return nil, 0, err
	}
	if headResp.Body != nil {
		headResp.Body.Close()
	}
	if headResp.StatusCode > 299 {
		return nil, 0, fmt.Errorf("unexpected HEAD status code %v: %s", headReq.String(), headResp.Status)
	}

	getReq := r.request(host, http.MethodGet, ps...)
	if err := getReq.addNamespace(r.refspec.Hostname()); err != nil {
		return nil, 0, err
	}
	return getReq, headResp.ContentLength, nil
}

func (r dockerFetcher) FetchByDigest(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchByDigestOpts) (io.ReadCloser, ocispec.Descriptor, error) {
	var desc ocispec.Descriptor
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", dgst))
	var config remotes.FetchByDigestConfig
	for _, o := range opts {
		if err := o(ctx, &config); err != nil {
			return nil, desc, err
		}
	}

	hosts := r.filterHosts(HostCapabilityPull)
	if len(hosts) == 0 {
		return nil, desc, fmt.Errorf("no pull hosts: %w", errdefs.ErrNotFound)
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, desc, err
	}

	var (
		getReq   *request
		sz       int64
		firstErr error
	)

	for i, host := range r.hosts {
		getReq, sz, err = r.createGetReq(ctx, host, i == len(r.hosts)-1, config.Mediatype, "blobs", dgst.String())
		if err == nil {
			break
		}
		// Store the error for referencing later
		if firstErr == nil {
			firstErr = err
		}
	}

	if getReq == nil {
		// Fall back to the "manifests" endpoint
		for i, host := range r.hosts {
			getReq, sz, err = r.createGetReq(ctx, host, i == len(r.hosts)-1, config.Mediatype, "manifests", dgst.String())
			if err == nil {
				break
			}
			// Store the error for referencing later
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if getReq == nil {
		if errdefs.IsNotFound(firstErr) {
			firstErr = fmt.Errorf("could not fetch content %v from remote: %w", dgst, errdefs.ErrNotFound)
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("could not fetch content %v from remote: (unknown)", dgst)
		}
		return nil, desc, firstErr
	}

	seeker, err := newHTTPReadSeeker(sz, func(offset int64) (io.ReadCloser, error) {
		return r.open(ctx, getReq, config.Mediatype, offset, true)
	})
	if err != nil {
		return nil, desc, err
	}

	desc = ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    dgst,
		Size:      sz,
	}
	if config.Mediatype != "" {
		desc.MediaType = config.Mediatype
	}
	return seeker, desc, nil
}

func (r dockerFetcher) open(ctx context.Context, req *request, mediatype string, offset int64, lastHost bool) (_ io.ReadCloser, retErr error) {
	const minChunkSize = 512

	chunkSize := int64(r.performances.ConcurrentLayerFetchBuffer)
	parallelism := int64(r.performances.MaxConcurrentDownloads)
	if chunkSize < minChunkSize || req.body != nil {
		parallelism = 1
	}
	log.G(ctx).WithField("initial_parallelism", r.performances.MaxConcurrentDownloads).
		WithField("parallelism", parallelism).
		WithField("chunk_size", chunkSize).
		WithField("offset", offset).
		Debug("fetching layer")
	req.setMediaType(mediatype)
	req.header.Set("Accept-Encoding", "zstd;q=1.0, gzip;q=0.8, deflate;q=0.5")
	if parallelism > 1 || offset > 0 {
		req.setOffset(offset)
	}

	if err := r.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	resp, err := req.doWithRetries(ctx, lastHost, withErrorCheck, withOffsetCheck(offset, parallelism))
	switch err {
	case nil:
		// all good
	case errContentRangeIgnored:
		if parallelism != 1 {
			log.G(ctx).WithError(err).Info("remote host ignored content range, forcing parallelism to 1")
			parallelism = 1
		}
	default:
		log.G(ctx).WithError(err).Debug("fetch failed")
		r.Release(1)
		return nil, err
	}

	body := &fnOnClose{
		BeforeClose: func() {
			r.Release(1)
		},
		ReadCloser: resp.Body,
	}
	defer func() {
		if retErr != nil {
			body.Close()
		}
	}()

	encoding := strings.FieldsFunc(resp.Header.Get("Content-Encoding"), func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})

	remaining, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 0)
	if remaining <= chunkSize {
		parallelism = 1
	}

	if parallelism > 1 {
		// If we have a content length, we can use multiple requests to fetch
		// the content in parallel. This will make download of bigger bodies
		// faster, at the cost of parallelism more requests and max
		// ~(max_parallelism * goroutine footprint) memory usage. The goroutine
		// footprint should be: the goroutine stack + pipe buffer size
		numChunks := remaining / chunkSize
		if numChunks*chunkSize < remaining {
			numChunks++
		}
		if numChunks < parallelism {
			parallelism = numChunks
		}
		queue := make(chan int64, parallelism)
		ctx, cancelCtx := context.WithCancel(ctx)
		done := ctx.Done()
		readers, writers := make([]io.Reader, numChunks), make([]*pipeWriter, numChunks)
		bufPool := newbufferPool(chunkSize)
		for i := range numChunks {
			readers[i], writers[i] = newPipeWriter(bufPool)
		}
		// keep reference of the initial body value to ensure it is closed
		ibody := body
		go func() {
			for i := range numChunks {
				select {
				case queue <- i:
				case <-done:
					if i == 0 {
						ibody.Close()
					}
					return // avoid leaking a goroutine if we exit early.
				}
			}
			close(queue)
		}()
		for range parallelism {
			go func() {
				for i := range queue { // first in first out
					copy := func() error {
						var body io.ReadCloser
						if i == 0 {
							body = ibody
						} else {
							if err := r.Acquire(ctx, 1); err != nil {
								return err
							}
							defer r.Release(1)
							reqClone := req.clone()
							reqClone.setOffset(offset + i*chunkSize)
							nresp, err := reqClone.doWithRetries(ctx, lastHost, withErrorCheck)
							if err != nil {
								_ = writers[i].CloseWithError(err)
								select {
								case <-done:
									return ctx.Err()
								default:
									cancelCtx()
								}
								return err
							}
							body = nresp.Body
						}
						_, err := io.Copy(writers[i], io.LimitReader(body, chunkSize))
						_ = body.Close()
						_ = writers[i].CloseWithError(err)
						if err != nil && err != io.EOF {
							cancelCtx()
							return err
						}
						return nil
					}
					if copy() != nil {
						return
					}
				}
			}()
		}
		body = &fnOnClose{
			BeforeClose: func() {
				cancelCtx()
			},
			ReadCloser: io.NopCloser(io.MultiReader(readers...)),
		}
	}

	for i := len(encoding) - 1; i >= 0; i-- {
		algorithm := strings.ToLower(encoding[i])
		switch algorithm {
		case "zstd":
			r, err := zstd.NewReader(body.ReadCloser,
				zstd.WithDecoderLowmem(false),
			)
			if err != nil {
				return nil, err
			}
			body.ReadCloser = r.IOReadCloser()
		case "gzip":
			r, err := gzip.NewReader(body.ReadCloser)
			if err != nil {
				return nil, err
			}
			body.ReadCloser = r
		case "deflate":
			body.ReadCloser = flate.NewReader(body.ReadCloser)
		case "identity", "":
			// no content-encoding applied, use raw body
		default:
			return nil, errors.New("unsupported Content-Encoding algorithm: " + algorithm)
		}
	}

	return body, nil
}

type fnOnClose struct {
	BeforeClose func()
	io.ReadCloser
}

// Close calls the BeforeClose function before closing the underlying
// ReadCloser.
func (f *fnOnClose) Close() error {
	f.BeforeClose()
	return f.ReadCloser.Close()
}

var _ io.ReadCloser = &fnOnClose{}
