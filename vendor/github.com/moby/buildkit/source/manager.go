package source

import (
	"context"
	"sync"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/pkg/errors"
)

type Source interface {
	ID() string
	Resolve(ctx context.Context, id Identifier, sm *session.Manager, vtx solver.Vertex) (SourceInstance, error)
}

type SourceInstance interface {
	CacheKey(ctx context.Context, g session.Group, index int) (string, solver.CacheOpts, bool, error)
	Snapshot(ctx context.Context, g session.Group) (cache.ImmutableRef, error)
}

type Manager struct {
	mu      sync.Mutex
	sources map[string]Source
}

func NewManager() (*Manager, error) {
	return &Manager{
		sources: make(map[string]Source),
	}, nil
}

func (sm *Manager) Register(src Source) {
	sm.mu.Lock()
	sm.sources[src.ID()] = src
	sm.mu.Unlock()
}

func (sm *Manager) Resolve(ctx context.Context, id Identifier, sessM *session.Manager, vtx solver.Vertex) (SourceInstance, error) {
	sm.mu.Lock()
	src, ok := sm.sources[id.ID()]
	sm.mu.Unlock()

	if !ok {
		return nil, errors.Errorf("no handler for %s", id.ID())
	}

	return src.Resolve(ctx, id, sessM, vtx)
}
