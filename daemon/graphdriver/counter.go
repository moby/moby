package graphdriver

import "sync"

// RefCounter is a generic counter for use by graphdriver Get/Put calls
type RefCounter struct {
	counts map[string]int
	mu     sync.Mutex
}

// NewRefCounter returns a new RefCounter
func NewRefCounter() *RefCounter {
	return &RefCounter{counts: make(map[string]int)}
}

// Increment increaes the ref count for the given id and returns the current count
func (c *RefCounter) Increment(id string) int {
	c.mu.Lock()
	c.counts[id]++
	count := c.counts[id]
	c.mu.Unlock()
	return count
}

// Decrement decreases the ref count for the given id and returns the current count
func (c *RefCounter) Decrement(id string) int {
	c.mu.Lock()
	c.counts[id]--
	count := c.counts[id]
	c.mu.Unlock()
	return count
}
