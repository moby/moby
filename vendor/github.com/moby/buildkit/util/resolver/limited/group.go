package limited

import (
	"context"
	"io"
	"runtime"
	"strings"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var Default = New(4)

type Group struct {
	mu   sync.Mutex
	size int
	sem  map[string][2]*semaphore.Weighted
}

type req struct {
	g   *Group
	ref string
}

func (r *req) acquire(ctx context.Context, desc ocispecs.Descriptor) (func(), error) {
	// json request get one additional connection
	highPriority := strings.HasSuffix(desc.MediaType, "+json")

	r.g.mu.Lock()
	s, ok := r.g.sem[r.ref]
	if !ok {
		s = [2]*semaphore.Weighted{
			semaphore.NewWeighted(int64(r.g.size)),
			semaphore.NewWeighted(int64(r.g.size + 1)),
		}
		r.g.sem[r.ref] = s
	}
	r.g.mu.Unlock()
	if !highPriority {
		if err := s[0].Acquire(ctx, 1); err != nil {
			return nil, err
		}
	}
	if err := s[1].Acquire(ctx, 1); err != nil {
		if !highPriority {
			s[0].Release(1)
		}
		return nil, err
	}
	return func() {
		s[1].Release(1)
		if !highPriority {
			s[0].Release(1)
		}
	}, nil
}

func New(size int) *Group {
	return &Group{
		size: size,
		sem:  make(map[string][2]*semaphore.Weighted),
	}
}

func (g *Group) req(ref string) *req {
	return &req{g: g, ref: domain(ref)}
}

func (g *Group) WrapFetcher(f remotes.Fetcher, ref string) remotes.Fetcher {
	return &fetcher{Fetcher: f, req: g.req(ref)}
}

func (g *Group) WrapPusher(p remotes.Pusher, ref string) remotes.Pusher {
	return &pusher{Pusher: p, req: g.req(ref)}
}

type pusher struct {
	remotes.Pusher
	req *req
}

func (p *pusher) Push(ctx context.Context, desc ocispecs.Descriptor) (content.Writer, error) {
	release, err := p.req.acquire(ctx, desc)
	if err != nil {
		return nil, err
	}
	w, err := p.Pusher.Push(ctx, desc)
	if err != nil {
		release()
		return nil, err
	}
	ww := &writer{Writer: w}
	closer := func() {
		if !ww.closed {
			logrus.Warnf("writer not closed cleanly: %s", desc.Digest)
		}
		release()
	}
	ww.release = closer
	runtime.SetFinalizer(ww, func(rc *writer) {
		rc.close()
	})
	return ww, nil
}

type writer struct {
	content.Writer
	once    sync.Once
	release func()
	closed  bool
}

func (w *writer) Close() error {
	w.closed = true
	w.close()
	return w.Writer.Close()
}

func (w *writer) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	w.closed = true
	w.close()
	return w.Writer.Commit(ctx, size, expected, opts...)
}

func (w *writer) close() {
	w.once.Do(w.release)
}

type fetcher struct {
	remotes.Fetcher
	req *req
}

func (f *fetcher) Fetch(ctx context.Context, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	release, err := f.req.acquire(ctx, desc)
	if err != nil {
		return nil, err
	}
	rc, err := f.Fetcher.Fetch(ctx, desc)
	if err != nil {
		release()
		return nil, err
	}

	rcw := &readCloser{ReadCloser: rc}
	closer := func() {
		if !rcw.closed {
			logrus.Warnf("fetcher not closed cleanly: %s", desc.Digest)
		}
		release()
	}
	rcw.release = closer
	runtime.SetFinalizer(rcw, func(rc *readCloser) {
		rc.close()
	})

	if s, ok := rc.(io.Seeker); ok {
		return &readCloserSeeker{rcw, s}, nil
	}

	return rcw, nil
}

type readCloserSeeker struct {
	*readCloser
	io.Seeker
}

type readCloser struct {
	io.ReadCloser
	once    sync.Once
	closed  bool
	release func()
}

func (r *readCloser) Close() error {
	r.closed = true
	r.close()
	return r.ReadCloser.Close()
}

func (r *readCloser) close() {
	r.once.Do(r.release)
}

func FetchHandler(ingester content.Ingester, fetcher remotes.Fetcher, ref string) images.HandlerFunc {
	return remotes.FetchHandler(ingester, Default.WrapFetcher(fetcher, ref))
}

func PushHandler(pusher remotes.Pusher, provider content.Provider, ref string) images.HandlerFunc {
	return remotes.PushHandler(Default.WrapPusher(pusher, ref), provider)
}

func domain(ref string) string {
	if ref != "" {
		if named, err := reference.ParseNormalizedNamed(ref); err == nil {
			return reference.Domain(named)
		}
	}
	return ref
}
