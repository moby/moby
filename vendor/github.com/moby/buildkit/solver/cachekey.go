package solver

import (
	"sync"

	digest "github.com/opencontainers/go-digest"
)

// NewCacheKey creates a new cache key for a specific output index
func NewCacheKey(dgst, vtx digest.Digest, output Index) *CacheKey {
	return &CacheKey{
		ID:     rootKey(dgst, output).String(),
		digest: dgst,
		vtx:    vtx,
		output: output,
		ids:    map[*cacheManager]string{},
	}
}

// CacheKeyWithSelector combines a cache key with an optional selector digest.
// Used to limit the matches for dependency cache key.
type CacheKeyWithSelector struct {
	Selector digest.Digest
	CacheKey ExportableCacheKey
}

type CacheKey struct {
	mu sync.RWMutex

	ID     string
	deps   [][]CacheKeyWithSelector // only [][]*inMemoryCacheKey
	digest digest.Digest
	vtx    digest.Digest
	output Index
	ids    map[*cacheManager]string

	indexIDs []string
}

func (ck *CacheKey) Deps() [][]CacheKeyWithSelector {
	ck.mu.RLock()
	defer ck.mu.RUnlock()
	deps := make([][]CacheKeyWithSelector, len(ck.deps))
	for i := range ck.deps {
		deps[i] = append([]CacheKeyWithSelector(nil), ck.deps[i]...)
	}
	return deps
}

func (ck *CacheKey) Digest() digest.Digest {
	return ck.digest
}
func (ck *CacheKey) Output() Index {
	return ck.output
}

func (ck *CacheKey) clone() *CacheKey {
	nk := &CacheKey{
		ID:     ck.ID,
		digest: ck.digest,
		vtx:    ck.vtx,
		output: ck.output,
		ids:    map[*cacheManager]string{},
	}
	for cm, id := range ck.ids {
		nk.ids[cm] = id
	}
	return nk
}
