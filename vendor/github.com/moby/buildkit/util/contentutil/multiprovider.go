package contentutil

import (
	"context"
	"sync"

	"github.com/containerd/containerd/v2/core/content"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/buildkit/session"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// NewMultiProvider creates a new mutable provider with a base provider
func NewMultiProvider(base content.InfoReaderProvider) *MultiProvider {
	return &MultiProvider{
		base: base,
		sub:  map[digest.Digest]content.InfoReaderProvider{},
	}
}

type sessionProvider interface {
	ProviderForSession(session.Group) content.Provider
}

// ProviderForSession provides a session-bound content provider when supported.
func ProviderForSession(p content.Provider, g session.Group) content.Provider {
	if sp, ok := p.(sessionProvider); ok {
		return sp.ProviderForSession(g)
	}
	return p
}

// MultiProvider is a provider backed by a mutable map of providers
type MultiProvider struct {
	mu   sync.RWMutex
	base content.InfoReaderProvider
	sub  map[digest.Digest]content.InfoReaderProvider
}

func (mp *MultiProvider) SnapshotLabels(descs []ocispecs.Descriptor, index int) map[string]string {
	if len(descs) < index {
		return nil
	}
	desc := descs[index]
	type snapshotLabels interface {
		SnapshotLabels([]ocispecs.Descriptor, int) map[string]string
	}

	mp.mu.RLock()
	if p, ok := mp.sub[desc.Digest]; ok {
		mp.mu.RUnlock()
		if cd, ok := p.(snapshotLabels); ok {
			return cd.SnapshotLabels(descs, index)
		}
	} else {
		mp.mu.RUnlock()
	}
	if cd, ok := mp.base.(snapshotLabels); ok {
		return cd.SnapshotLabels(descs, index)
	}
	return nil
}

// ReaderAt returns a content.ReaderAt
func (mp *MultiProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	mp.mu.RLock()
	if p, ok := mp.sub[desc.Digest]; ok {
		mp.mu.RUnlock()
		return p.ReaderAt(ctx, desc)
	}
	mp.mu.RUnlock()
	if mp.base == nil {
		return nil, errors.Wrapf(cerrdefs.ErrNotFound, "content %v", desc.Digest)
	}
	return mp.base.ReaderAt(ctx, desc)
}

// Info returns a content.Info
func (mp *MultiProvider) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	mp.mu.RLock()
	if p, ok := mp.sub[dgst]; ok {
		mp.mu.RUnlock()
		return p.Info(ctx, dgst)
	}
	mp.mu.RUnlock()
	if mp.base == nil {
		return content.Info{}, errors.Wrapf(cerrdefs.ErrNotFound, "content %v", dgst)
	}
	return mp.base.Info(ctx, dgst)
}

// Add adds a new child provider for a specific digest
func (mp *MultiProvider) Add(dgst digest.Digest, p content.InfoReaderProvider) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.sub[dgst] = p
}

func (mp *MultiProvider) ProviderForSession(g session.Group) content.Provider {
	return &multiProviderForSession{MultiProvider: mp, g: g}
}

type multiProviderForSession struct {
	*MultiProvider
	g session.Group
}

func (p *multiProviderForSession) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	p.mu.RLock()
	provider, ok := p.sub[desc.Digest]
	if !ok {
		provider = p.base
	}
	p.mu.RUnlock()
	if provider == nil {
		return nil, errors.Wrapf(cerrdefs.ErrNotFound, "content %v", desc.Digest)
	}
	return ProviderForSession(provider, p.g).ReaderAt(ctx, desc)
}

func (mp *MultiProvider) UnlazySession(desc ocispecs.Descriptor) session.Group {
	type unlazySession interface {
		UnlazySession(ocispecs.Descriptor) session.Group
	}

	mp.mu.RLock()
	if p, ok := mp.sub[desc.Digest]; ok {
		mp.mu.RUnlock()
		if cd, ok := p.(unlazySession); ok {
			return cd.UnlazySession(desc)
		}
	} else {
		mp.mu.RUnlock()
	}
	if cd, ok := mp.base.(unlazySession); ok {
		return cd.UnlazySession(desc)
	}
	return nil
}
