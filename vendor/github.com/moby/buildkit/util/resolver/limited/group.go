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
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

type contextKeyT string

var contextKey = contextKeyT("buildkit/util/resolver/limited")

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

func (r *req) acquire(ctx context.Context, desc ocispecs.Descriptor) (context.Context, func(), error) {
	if v := ctx.Value(contextKey); v != nil {
		return ctx, func() {}, nil
	}

	ctx = context.WithValue(ctx, contextKey, struct{}{})

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
			return ctx, nil, err
		}
	}
	if err := s[1].Acquire(ctx, 1); err != nil {
		if !highPriority {
			s[0].Release(1)
		}
		return ctx, nil, err
	}
	return ctx, func() {
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

func (g *Group) PushHandler(pusher remotes.Pusher, provider content.Provider, ref string) images.HandlerFunc {
	ph := remotes.PushHandler(pusher, provider)
	req := g.req(ref)
	return func(ctx context.Context, desc ocispecs.Descriptor) ([]ocispecs.Descriptor, error) {
		ctx, release, err := req.acquire(ctx, desc)
		if err != nil {
			return nil, err
		}
		defer release()
		return ph(ctx, desc)
	}
}

type fetcher struct {
	remotes.Fetcher
	req *req
}

func (f *fetcher) Fetch(ctx context.Context, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	ctx, release, err := f.req.acquire(ctx, desc)
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
	return Default.PushHandler(pusher, provider, ref)
}

func domain(ref string) string {
	if ref != "" {
		if named, err := reference.ParseNormalizedNamed(ref); err == nil {
			return reference.Domain(named)
		}
	}
	return ref
}
