package source

import (
	"context"
	"strings"
	"sync"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

// Source implementations provide "root" vertices in the graph that can be
// constructed from a URI-like string and arbitrary attrs.
type Source interface {
	// Schemes returns a list of SourceOp identifier schemes that this source
	// should match.
	Schemes() []string

	// Identifier constructs an Identifier from the given scheme, ref, and attrs,
	// all of which come from a SourceOp.
	Identifier(scheme, ref string, attrs map[string]string, platform *pb.Platform) (Identifier, error)

	// Resolve constructs an instance of the source from an Identifier.
	Resolve(ctx context.Context, id Identifier, sm *session.Manager, vtx solver.Vertex) (SourceInstance, error)
}

// SourceInstance represents a cacheable vertex created by a Source.
type SourceInstance interface {
	// CacheKey returns the cache key for the instance.
	CacheKey(ctx context.Context, g session.Group, index int) (key, pin string, opts solver.CacheOpts, done bool, err error)

	// Snapshot creates a cache ref for the instance. May return a nil ref if source points to empty content, e.g. image without any layers.
	Snapshot(ctx context.Context, g session.Group) (cache.ImmutableRef, error)
}

type Manager struct {
	mu      sync.Mutex
	schemes map[string]Source
}

func NewManager() (*Manager, error) {
	return &Manager{
		schemes: make(map[string]Source),
	}, nil
}

func (sm *Manager) Register(src Source) {
	sm.mu.Lock()
	for _, scheme := range src.Schemes() {
		sm.schemes[scheme] = src
	}
	sm.mu.Unlock()
}

func (sm *Manager) Identifier(op *pb.Op_Source, platform *pb.Platform) (Identifier, error) {
	scheme, ref, ok := strings.Cut(op.Source.Identifier, "://")
	if !ok {
		return nil, errors.Wrapf(errInvalid, "failed to parse %s", op.Source.Identifier)
	}

	sm.mu.Lock()
	source, found := sm.schemes[scheme]
	sm.mu.Unlock()

	if !found {
		return nil, errors.Wrapf(errNotFound, "unknown scheme %s", scheme)
	}

	return source.Identifier(scheme, ref, op.Source.Attrs, platform)
}

func (sm *Manager) Resolve(ctx context.Context, id Identifier, sessM *session.Manager, vtx solver.Vertex) (SourceInstance, error) {
	sm.mu.Lock()
	src, ok := sm.schemes[id.Scheme()]
	sm.mu.Unlock()

	if !ok {
		return nil, errors.Errorf("no handler for %s", id.Scheme())
	}

	return src.Resolve(ctx, id, sessM, vtx)
}
