package solver

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SharedResult is a result that can be cloned
type SharedResult struct {
	mu   sync.Mutex
	main Result
}

func NewSharedResult(main Result) *SharedResult {
	return &SharedResult{main: main}
}

func (r *SharedResult) Clone() Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	r1, r2 := dup(r.main)
	r.main = r1
	return r2
}

func (r *SharedResult) Release(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.main.Release(ctx)
}

func dup(res Result) (Result, Result) {
	sem := int64(0)
	return &splitResult{Result: res, sem: &sem}, &splitResult{Result: res, sem: &sem}
}

type splitResult struct {
	Result
	released int64
	sem      *int64
}

func (r *splitResult) Release(ctx context.Context) error {
	if atomic.AddInt64(&r.released, 1) > 1 {
		err := errors.Errorf("releasing already released reference")
		logrus.Error(err)
		return err
	}
	if atomic.AddInt64(r.sem, 1) == 2 {
		return r.Result.Release(ctx)
	}
	return nil
}

// NewCachedResult combines a result and cache key into cached result
func NewCachedResult(res Result, k []ExportableCacheKey) CachedResult {
	return &cachedResult{res, k}
}

type cachedResult struct {
	Result
	k []ExportableCacheKey
}

func (cr *cachedResult) CacheKeys() []ExportableCacheKey {
	return cr.k
}

func NewSharedCachedResult(res CachedResult) *SharedCachedResult {
	return &SharedCachedResult{
		SharedResult: NewSharedResult(res),
		CachedResult: res,
	}
}

func (r *SharedCachedResult) Clone() CachedResult {
	return &clonedCachedResult{Result: r.SharedResult.Clone(), cr: r.CachedResult}
}

func (r *SharedCachedResult) Release(ctx context.Context) error {
	return r.SharedResult.Release(ctx)
}

type clonedCachedResult struct {
	Result
	cr CachedResult
}

func (r *clonedCachedResult) ID() string {
	return r.Result.ID()
}

func (cr *clonedCachedResult) CacheKeys() []ExportableCacheKey {
	return cr.cr.CacheKeys()
}

type SharedCachedResult struct {
	*SharedResult
	CachedResult
}
