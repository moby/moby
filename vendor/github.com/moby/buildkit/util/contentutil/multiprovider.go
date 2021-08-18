package contentutil

import (
	"context"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
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
