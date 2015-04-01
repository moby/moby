// Package cache provides facilities to speed up access to the storage
// backend. Typically cache implementations deal with internal implementation
// details at the backend level, rather than generalized caches for
// distribution related interfaces. In other words, unless the cache is
// specific to the storage package, it belongs in another package.
package cache

import (
	"fmt"

	"github.com/docker/distribution/digest"
	"golang.org/x/net/context"
)

// ErrNotFound is returned when a meta item is not found.
var ErrNotFound = fmt.Errorf("not found")

// LayerMeta describes the backend location and length of layer data.
type LayerMeta struct {
	Path   string
	Length int64
}

// LayerInfoCache is a driver-aware cache of layer metadata. Basically, it
// provides a fast cache for checks against repository metadata, avoiding
// round trips to backend storage. Note that this is different from a pure
// layer cache, which would also provide access to backing data, as well. Such
// a cache should be implemented as a middleware, rather than integrated with
// the storage backend.
//
// Note that most implementations rely on the caller to do strict checks on on
// repo and dgst arguments, since these are mostly used behind existing
// implementations.
type LayerInfoCache interface {
	// Contains returns true if the repository with name contains the layer.
	Contains(ctx context.Context, repo string, dgst digest.Digest) (bool, error)

	// Add includes the layer in the given repository cache.
	Add(ctx context.Context, repo string, dgst digest.Digest) error

	// Meta provides the location of the layer on the backend and its size. Membership of a
	// repository should be tested before using the result, if required.
	Meta(ctx context.Context, dgst digest.Digest) (LayerMeta, error)

	// SetMeta sets the meta data for the given layer.
	SetMeta(ctx context.Context, dgst digest.Digest, meta LayerMeta) error
}

// base implements common checks between cache implementations. Note that
// these are not full checks of input, since that should be done by the
// caller.
type base struct {
	LayerInfoCache
}

func (b *base) Contains(ctx context.Context, repo string, dgst digest.Digest) (bool, error) {
	if repo == "" {
		return false, fmt.Errorf("cache: cannot check for empty repository name")
	}

	if dgst == "" {
		return false, fmt.Errorf("cache: cannot check for empty digests")
	}

	return b.LayerInfoCache.Contains(ctx, repo, dgst)
}

func (b *base) Add(ctx context.Context, repo string, dgst digest.Digest) error {
	if repo == "" {
		return fmt.Errorf("cache: cannot add empty repository name")
	}

	if dgst == "" {
		return fmt.Errorf("cache: cannot add empty digest")
	}

	return b.LayerInfoCache.Add(ctx, repo, dgst)
}

func (b *base) Meta(ctx context.Context, dgst digest.Digest) (LayerMeta, error) {
	if dgst == "" {
		return LayerMeta{}, fmt.Errorf("cache: cannot get meta for empty digest")
	}

	return b.LayerInfoCache.Meta(ctx, dgst)
}

func (b *base) SetMeta(ctx context.Context, dgst digest.Digest, meta LayerMeta) error {
	if dgst == "" {
		return fmt.Errorf("cache: cannot set meta for empty digest")
	}

	if meta.Path == "" {
		return fmt.Errorf("cache: cannot set empty path for meta")
	}

	return b.LayerInfoCache.SetMeta(ctx, dgst, meta)
}
