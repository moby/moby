package solver

import (
	"slices"
	"sync"
)

type resolverCache struct {
	mu    sync.Mutex
	locks map[any]*entry
}

var _ ResolverCache = &resolverCache{}

type entry struct {
	waiting []chan struct{}
	values  []any
	locked  bool
}

func newResolverCache() *resolverCache {
	return &resolverCache{locks: make(map[any]*entry)}
}

func (r *resolverCache) Lock(key any) (values []any, release func(any) error, err error) {
	r.mu.Lock()
	e, ok := r.locks[key]
	if !ok {
		e = &entry{}
		r.locks[key] = e
	}
	if !e.locked {
		e.locked = true
		values = slices.Clone(e.values)
		r.mu.Unlock()
		return values, func(v any) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			if v != nil {
				e.values = append(e.values, v)
			}
			for _, ch := range e.waiting {
				close(ch)
			}
			e.waiting = nil
			e.locked = false
			if len(e.values) == 0 {
				delete(r.locks, key)
			}
			return nil
		}, nil
	}

	ch := make(chan struct{})
	e.waiting = append(e.waiting, ch)
	r.mu.Unlock()

	<-ch // wait for unlock

	r.mu.Lock()
	defer r.mu.Unlock()
	e2, ok := r.locks[key]
	if !ok {
		return nil, nil, nil // key deleted
	}
	values = slices.Clone(e2.values)
	if e2.locked {
		// shouldn't happen, but protect against logic errors
		return values, func(any) error { return nil }, nil
	}
	e2.locked = true
	return values, func(v any) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if v != nil {
			e2.values = append(e2.values, v)
		}
		for _, ch := range e2.waiting {
			close(ch)
		}
		e2.waiting = nil
		e2.locked = false
		if len(e2.values) == 0 {
			delete(r.locks, key)
		}
		return nil
	}, nil
}

// combinedResolverCache returns a ResolverCache that wraps multiple caches.
// Lock() calls each underlying cache in parallel, merges their values, and
// returns a combined release that releases all sublocks.
func combinedResolverCache(rcs []ResolverCache) ResolverCache {
	return &combinedCache{rcs: rcs}
}

type combinedCache struct {
	rcs []ResolverCache
}

func (c *combinedCache) Lock(key any) (values []any, release func(any) error, err error) {
	if len(c.rcs) == 0 {
		return nil, func(any) error { return nil }, nil
	}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		valuesAll []any
		releasers []func(any) error
		firstErr  error
	)

	wg.Add(len(c.rcs))
	for _, rc := range c.rcs {
		go func(rc ResolverCache) {
			defer wg.Done()
			vals, rel, e := rc.Lock(key)
			if e != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = e
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			valuesAll = append(valuesAll, vals...)
			releasers = append(releasers, rel)
			mu.Unlock()
		}(rc)
	}

	wg.Wait()

	if firstErr != nil {
		// rollback all acquired locks
		for _, r := range releasers {
			_ = r(nil)
		}
		return nil, nil, firstErr
	}

	release = func(v any) error {
		var errOnce error
		for _, r := range releasers {
			if e := r(v); e != nil && errOnce == nil {
				errOnce = e
			}
		}
		return errOnce
	}

	return valuesAll, release, nil
}
