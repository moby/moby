package contentutil

import (
	"bytes"
	"context"
	"io/ioutil"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Buffer is a content provider and ingester that keeps data in memory
type Buffer interface {
	content.Provider
	content.Ingester
}

// NewBuffer returns a new buffer
func NewBuffer() Buffer {
	return &buffer{
		buffers: map[digest.Digest][]byte{},
		refs:    map[string]struct{}{},
	}
}

type buffer struct {
	mu      sync.Mutex
	buffers map[digest.Digest][]byte
	refs    map[string]struct{}
}

func (b *buffer) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return nil, err
		}
	}
	b.mu.Lock()
	if _, ok := b.refs[wOpts.Ref]; ok {
		return nil, errors.Wrapf(errdefs.ErrUnavailable, "ref %s locked", wOpts.Ref)
	}
	b.mu.Unlock()
	return &bufferedWriter{
		main:     b,
		digester: digest.Canonical.Digester(),
		buffer:   bytes.NewBuffer(nil),
		expected: wOpts.Desc.Digest,
		releaseRef: func() {
			b.mu.Lock()
			delete(b.refs, wOpts.Ref)
			b.mu.Unlock()
		},
	}, nil
}

func (b *buffer) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	r, err := b.getBytesReader(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	return &readerAt{Reader: r, Closer: ioutil.NopCloser(r), size: int64(r.Len())}, nil
}

func (b *buffer) getBytesReader(ctx context.Context, dgst digest.Digest) (*bytes.Reader, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if dt, ok := b.buffers[dgst]; ok {
		return bytes.NewReader(dt), nil
	}

	return nil, errors.Wrapf(errdefs.ErrNotFound, "content %v", dgst)
}

func (b *buffer) addValue(k digest.Digest, dt []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffers[k] = dt
}

type bufferedWriter struct {
	main       *buffer
	ref        string
	offset     int64
	total      int64
	startedAt  time.Time
	updatedAt  time.Time
	buffer     *bytes.Buffer
	expected   digest.Digest
	digester   digest.Digester
	releaseRef func()
}

func (w *bufferedWriter) Write(p []byte) (n int, err error) {
	n, err = w.buffer.Write(p)
	w.digester.Hash().Write(p[:n])
	w.offset += int64(len(p))
	w.updatedAt = time.Now()
	return n, err
}

func (w *bufferedWriter) Close() error {
	if w.buffer != nil {
		w.releaseRef()
		w.buffer = nil
	}
	return nil
}

func (w *bufferedWriter) Status() (content.Status, error) {
	return content.Status{
		Ref:       w.ref,
		Offset:    w.offset,
		Total:     w.total,
		StartedAt: w.startedAt,
		UpdatedAt: w.updatedAt,
	}, nil
}

func (w *bufferedWriter) Digest() digest.Digest {
	return w.digester.Digest()
}

func (w *bufferedWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opt ...content.Opt) error {
	if w.buffer == nil {
		return errors.Errorf("can't commit already committed or closed")
	}
	if s := int64(w.buffer.Len()); size > 0 && size != s {
		return errors.Errorf("unexpected commit size %d, expected %d", s, size)
	}
	dgst := w.digester.Digest()
	if expected != "" && expected != dgst {
		return errors.Errorf("unexpected digest: %v != %v", dgst, expected)
	}
	if w.expected != "" && w.expected != dgst {
		return errors.Errorf("unexpected digest: %v != %v", dgst, w.expected)
	}
	w.main.addValue(dgst, w.buffer.Bytes())
	return w.Close()
}

func (w *bufferedWriter) Truncate(size int64) error {
	if size != 0 {
		return errors.New("Truncate: unsupported size")
	}
	w.offset = 0
	w.digester.Hash().Reset()
	w.buffer.Reset()
	return nil
}
