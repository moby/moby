package pools

import "sync"

// Pool provides a typed wrapper around sync.Pool.
type Pool[T any] struct {
	pool sync.Pool
}

// New returns a typed pool backed by sync.Pool.
func New[T any](newFn func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

// Get returns a pooled value.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns a value to the pool.
func (p *Pool[T]) Put(v T) {
	p.pool.Put(v)
}
