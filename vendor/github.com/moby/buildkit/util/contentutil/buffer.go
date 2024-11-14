package contentutil

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	cerrdefs "github.com/containerd/errdefs"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Buffer is a content provider and ingester that keeps data in memory
type Buffer interface {
	content.Provider
	content.Ingester
	content.Manager
}

// NewBuffer returns a new buffer
func NewBuffer() Buffer {
	return &buffer{
		buffers: map[digest.Digest][]byte{},
		infos:   map[digest.Digest]content.Info{},
		refs:    map[string]struct{}{},
	}
}

type buffer struct {
	mu      sync.Mutex
	buffers map[digest.Digest][]byte
	infos   map[digest.Digest]content.Info
	refs    map[string]struct{}
}

func (b *buffer) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	b.mu.Lock()
	v, ok := b.infos[dgst]
	b.mu.Unlock()
	if !ok {
		return content.Info{}, cerrdefs.ErrNotFound
	}
	return v, nil
}

func (b *buffer) Update(ctx context.Context, new content.Info, fieldpaths ...string) (content.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	updated, ok := b.infos[new.Digest]
	if !ok {
		return content.Info{}, cerrdefs.ErrNotFound
	}

	if len(fieldpaths) == 0 {
		fieldpaths = []string{"labels"}
	}

	for _, path := range fieldpaths {
		if strings.HasPrefix(path, "labels.") {
			if updated.Labels == nil {
				updated.Labels = map[string]string{}
			}
			key := strings.TrimPrefix(path, "labels.")
			updated.Labels[key] = new.Labels[key]
			continue
		}
		if path == "labels" {
			updated.Labels = new.Labels
		}
	}

	b.infos[new.Digest] = updated
	return updated, nil
}

func (b *buffer) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return nil // not implemented
}

func (b *buffer) Delete(ctx context.Context, dgst digest.Digest) error {
	return nil // not implemented
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
		return nil, errors.Wrapf(cerrdefs.ErrUnavailable, "ref %s locked", wOpts.Ref)
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

func (b *buffer) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	r, err := b.getBytesReader(desc.Digest)
	if err != nil {
		return nil, err
	}
	return &readerAt{Reader: r, Closer: io.NopCloser(r), size: int64(r.Len())}, nil
}

func (b *buffer) getBytesReader(dgst digest.Digest) (*bytes.Reader, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if dt, ok := b.buffers[dgst]; ok {
		return bytes.NewReader(dt), nil
	}

	return nil, errors.Wrapf(cerrdefs.ErrNotFound, "content %v", dgst)
}

func (b *buffer) addValue(k digest.Digest, dt []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffers[k] = dt
	b.infos[k] = content.Info{Digest: k, Size: int64(len(dt))}
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
