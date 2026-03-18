package flightcontrol

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

// Group is a flightcontrol synchronization group that memoizes the results of a function
// and returns the cached result if the function is called with the same key.
// Don't use with long-running groups as the results are cached indefinitely.
type CachedGroup[T any] struct {
	// CacheError defines if error results should also be cached.
	// It is not safe to change this value after the first call to Do.
	// Context cancellation errors are never cached.
	CacheError bool
	g          Group[T]
	mu         sync.Mutex
	cache      map[string]result[T]
}

type result[T any] struct {
	v   T
	err error
}

// Do executes a context function syncronized by the key or returns the cached result for the key.
func (g *CachedGroup[T]) Do(ctx context.Context, key string, fn func(ctx context.Context) (T, error)) (T, error) {
	return g.g.Do(ctx, key, func(ctx context.Context) (T, error) {
		g.mu.Lock()
		if v, ok := g.cache[key]; ok {
			g.mu.Unlock()
			if v.err != nil {
				if g.CacheError {
					return v.v, v.err
				}
			} else {
				return v.v, nil
			}
		}
		g.mu.Unlock()
		v, err := fn(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				if errors.Is(err, context.Cause(ctx)) {
					return v, err
				}
			default:
			}
		}
		if err == nil || g.CacheError {
			g.mu.Lock()
			if g.cache == nil {
				g.cache = make(map[string]result[T])
			}
			g.cache[key] = result[T]{v: v, err: err}
			g.mu.Unlock()
		}
		return v, err
	})
}
