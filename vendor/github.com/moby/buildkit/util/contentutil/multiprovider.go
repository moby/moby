package contentutil

import (
	"context"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/moby/buildkit/session"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// NewMultiProvider creates a new mutable provider with a base provider
func NewMultiProvider(base content.Provider) *MultiProvider {
	return &MultiProvider{
		base: base,
		sub:  map[digest.Digest]content.Provider{},
	}
}

// MultiProvider is a provider backed by a mutable map of providers
type MultiProvider struct {
	mu   sync.RWMutex
	base content.Provider
	sub  map[digest.Digest]content.Provider
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

func (mp *MultiProvider) CheckDescriptor(ctx context.Context, desc ocispecs.Descriptor) error {
	type checkDescriptor interface {
		CheckDescriptor(context.Context, ocispecs.Descriptor) error
	}

	mp.mu.RLock()
	if p, ok := mp.sub[desc.Digest]; ok {
		mp.mu.RUnlock()
		if cd, ok := p.(checkDescriptor); ok {
			return cd.CheckDescriptor(ctx, desc)
		}
	} else {
		mp.mu.RUnlock()
	}
	if cd, ok := mp.base.(checkDescriptor); ok {
		return cd.CheckDescriptor(ctx, desc)
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
		return nil, errors.Wrapf(errdefs.ErrNotFound, "content %v", desc.Digest)
	}
	return mp.base.ReaderAt(ctx, desc)
}

// Add adds a new child provider for a specific digest
func (mp *MultiProvider) Add(dgst digest.Digest, p content.Provider) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.sub[dgst] = p
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
