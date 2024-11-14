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

package content

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/internal/randutil"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var ErrReset = errors.New("writer has been reset")

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 1<<20)
		return &buffer
	},
}

type reader interface {
	Reader() io.Reader
}

// NewReader returns a io.Reader from a ReaderAt
func NewReader(ra ReaderAt) io.Reader {
	if rd, ok := ra.(reader); ok {
		return rd.Reader()
	}
	return io.NewSectionReader(ra, 0, ra.Size())
}

type nopCloserBytesReader struct {
	*bytes.Reader
}

func (*nopCloserBytesReader) Close() error { return nil }

type nopCloserSectionReader struct {
	*io.SectionReader
}

func (*nopCloserSectionReader) Close() error { return nil }

// BlobReadSeeker returns a read seeker for the blob from the provider.
func BlobReadSeeker(ctx context.Context, provider Provider, desc ocispec.Descriptor) (io.ReadSeekCloser, error) {
	if int64(len(desc.Data)) == desc.Size && digest.FromBytes(desc.Data) == desc.Digest {
		return &nopCloserBytesReader{bytes.NewReader(desc.Data)}, nil
	}

	ra, err := provider.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &nopCloserSectionReader{io.NewSectionReader(ra, 0, ra.Size())}, nil
}

// ReadBlob retrieves the entire contents of the blob from the provider.
//
// Avoid using this for large blobs, such as layers.
func ReadBlob(ctx context.Context, provider Provider, desc ocispec.Descriptor) ([]byte, error) {
	if int64(len(desc.Data)) == desc.Size && digest.FromBytes(desc.Data) == desc.Digest {
		return desc.Data, nil
	}

	ra, err := provider.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer ra.Close()

	p := make([]byte, ra.Size())

	n, err := ra.ReadAt(p, 0)
	if err == io.EOF {
		if int64(n) != ra.Size() {
			err = io.ErrUnexpectedEOF
		} else {
			err = nil
		}
	}
	return p, err
}

// WriteBlob writes data with the expected digest into the content store. If
// expected already exists, the method returns immediately and the reader will
// not be consumed.
//
// This is useful when the digest and size are known beforehand.
//
// Copy is buffered, so no need to wrap reader in buffered io.
func WriteBlob(ctx context.Context, cs Ingester, ref string, r io.Reader, desc ocispec.Descriptor, opts ...Opt) error {
	cw, err := OpenWriter(ctx, cs, WithRef(ref), WithDescriptor(desc))
	if err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("failed to open writer: %w", err)
		}

		return nil // already present
	}
	defer cw.Close()

	return Copy(ctx, cw, r, desc.Size, desc.Digest, opts...)
}

// OpenWriter opens a new writer for the given reference, retrying if the writer
// is locked until the reference is available or returns an error.
func OpenWriter(ctx context.Context, cs Ingester, opts ...WriterOpt) (Writer, error) {
	var (
		cw    Writer
		err   error
		retry = 16
	)
	for {
		cw, err = cs.Writer(ctx, opts...)
		if err != nil {
			if !errdefs.IsUnavailable(err) {
				return nil, err
			}

			// TODO: Check status to determine if the writer is active,
			// continue waiting while active, otherwise return lock
			// error or abort. Requires asserting for an ingest manager

			select {
			case <-time.After(time.Millisecond * time.Duration(randutil.Intn(retry))):
				if retry < 2048 {
					retry = retry << 1
				}
				continue
			case <-ctx.Done():
				// Propagate lock error
				return nil, err
			}

		}
		break
	}

	return cw, err
}

// Copy copies data with the expected digest from the reader into the
// provided content store writer. This copy commits the writer.
//
// This is useful when the digest and size are known beforehand. When
// the size or digest is unknown, these values may be empty.
//
// Copy is buffered, so no need to wrap reader in buffered io.
func Copy(ctx context.Context, cw Writer, or io.Reader, size int64, expected digest.Digest, opts ...Opt) error {
	r := or
	for i := 0; ; i++ {
		if i >= 1 {
			log.G(ctx).WithField("digest", expected).Debugf("retrying copy due to reset")
		}

		ws, err := cw.Status()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}
		// Reset the original reader if
		// 1. there is an offset, or
		// 2. this is a retry due to Reset error
		if ws.Offset > 0 || i > 0 {
			r, err = seekReader(or, ws.Offset, size)
			if err != nil {
				return fmt.Errorf("unable to resume write to %v: %w", ws.Ref, err)
			}
		}

		copied, err := copyWithBuffer(cw, r)
		if errors.Is(err, ErrReset) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		if size != 0 && copied < size-ws.Offset {
			// Short writes would return its own error, this indicates a read failure
			return fmt.Errorf("failed to read expected number of bytes: %w", io.ErrUnexpectedEOF)
		}
		if err := cw.Commit(ctx, size, expected, opts...); err != nil {
			if errors.Is(err, ErrReset) {
				continue
			}
			if !errdefs.IsAlreadyExists(err) {
				return fmt.Errorf("failed commit on ref %q: %w", ws.Ref, err)
			}
		}
		return nil
	}
}

// CopyReaderAt copies to a writer from a given reader at for the given
// number of bytes. This copy does not commit the writer.
func CopyReaderAt(cw Writer, ra ReaderAt, n int64) error {
	ws, err := cw.Status()
	if err != nil {
		return err
	}

	copied, err := copyWithBuffer(cw, io.NewSectionReader(ra, ws.Offset, n))
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	if copied < n {
		// Short writes would return its own error, this indicates a read failure
		return fmt.Errorf("failed to read expected number of bytes: %w", io.ErrUnexpectedEOF)
	}
	return nil
}

// CopyReader copies to a writer from a given reader, returning
// the number of bytes copied.
// Note: if the writer has a non-zero offset, the total number
// of bytes read may be greater than those copied if the reader
// is not an io.Seeker.
// This copy does not commit the writer.
func CopyReader(cw Writer, r io.Reader) (int64, error) {
	ws, err := cw.Status()
	if err != nil {
		return 0, fmt.Errorf("failed to get status: %w", err)
	}

	if ws.Offset > 0 {
		r, err = seekReader(r, ws.Offset, 0)
		if err != nil {
			return 0, fmt.Errorf("unable to resume write to %v: %w", ws.Ref, err)
		}
	}

	return copyWithBuffer(cw, r)
}

// seekReader attempts to seek the reader to the given offset, either by
// resolving `io.Seeker`, by detecting `io.ReaderAt`, or discarding
// up to the given offset.
func seekReader(r io.Reader, offset, size int64) (io.Reader, error) {
	// attempt to resolve r as a seeker and setup the offset.
	seeker, ok := r.(io.Seeker)
	if ok {
		nn, err := seeker.Seek(offset, io.SeekStart)
		if nn != offset {
			if err == nil {
				err = fmt.Errorf("unexpected seek location without seek error")
			}
			return nil, fmt.Errorf("failed to seek to offset %v: %w", offset, err)
		}

		if err != nil {
			return nil, err
		}

		return r, nil
	}

	// ok, let's try io.ReaderAt!
	readerAt, ok := r.(io.ReaderAt)
	if ok && size > offset {
		sr := io.NewSectionReader(readerAt, offset, size)
		return sr, nil
	}

	// well then, let's just discard up to the offset
	n, err := copyWithBuffer(io.Discard, io.LimitReader(r, offset))
	if err != nil {
		return nil, fmt.Errorf("failed to discard to offset: %w", err)
	}
	if n != offset {
		return nil, errors.New("unable to discard to offset")
	}

	return r, nil
}

// copyWithBuffer is very similar to  io.CopyBuffer https://golang.org/pkg/io/#CopyBuffer
// but instead of using Read to read from the src, we use ReadAtLeast to make sure we have
// a full buffer before we do a write operation to dst to reduce overheads associated
// with the write operations of small buffers.
func copyWithBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	bufRef := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufRef)
	buf := *bufRef
	for {
		nr, er := io.ReadAtLeast(src, buf, len(buf))
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			// If an EOF happens after reading fewer than the requested bytes,
			// ReadAtLeast returns ErrUnexpectedEOF.
			if er != io.EOF && er != io.ErrUnexpectedEOF {
				err = er
			}
			break
		}
	}
	return
}

// Exists returns whether an attempt to access the content would not error out
// with an ErrNotFound error. It will return an encountered error if it was
// different than ErrNotFound.
func Exists(ctx context.Context, provider InfoProvider, desc ocispec.Descriptor) (bool, error) {
	_, err := provider.Info(ctx, desc.Digest)
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}
