package distribution

import (
	"sync"

	"github.com/docker/docker/pkg/broadcaster"
)

// A Pool manages concurrent pulls. It deduplicates in-progress downloads.
type Pool struct {
	sync.Mutex
	pullingPool map[string]*broadcaster.Buffered
}

// NewPool creates a new Pool.
func NewPool() *Pool {
	return &Pool{
		pullingPool: make(map[string]*broadcaster.Buffered),
	}
}

// add checks if a pull is already running, and returns (broadcaster, true)
// if a running operation is found. Otherwise, it creates a new one and returns
// (broadcaster, false).
func (pool *Pool) add(key string) (*broadcaster.Buffered, bool) {
	pool.Lock()
	defer pool.Unlock()

	if p, exists := pool.pullingPool[key]; exists {
		return p, true
	}

	broadcaster := broadcaster.NewBuffered()
	pool.pullingPool[key] = broadcaster

	return broadcaster, false
}

func (pool *Pool) removeWithError(key string, broadcasterResult error) error {
	pool.Lock()
	defer pool.Unlock()
	if broadcaster, exists := pool.pullingPool[key]; exists {
		broadcaster.CloseWithError(broadcasterResult)
		delete(pool.pullingPool, key)
	}
	return nil
}

func (pool *Pool) remove(key string) error {
	return pool.removeWithError(key, nil)
}
