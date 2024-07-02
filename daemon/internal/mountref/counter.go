package mountref

import "sync"

type minfo struct {
	check bool
	count int
}

// Counter is a generic counter for use by graphdriver Get/Put calls
type Counter struct {
	counts    map[string]*minfo
	mu        sync.Mutex
	isMounted Checker
}

// Checker checks whether the provided path is mounted.
type Checker func(path string) bool

// NewCounter returns a new Counter. It accepts a [Checker] to
// determine whether a path is mounted.
func NewCounter(c Checker) *Counter {
	return &Counter{
		isMounted: c,
		counts:    make(map[string]*minfo),
	}
}

// Increment increases the ref count for the given id and returns the current count
func (c *Counter) Increment(path string) int {
	return c.incdec(path, func(minfo *minfo) {
		minfo.count++
	})
}

// Decrement decreases the ref count for the given id and returns the current count
func (c *Counter) Decrement(path string) int {
	return c.incdec(path, func(minfo *minfo) {
		minfo.count--
	})
}

func (c *Counter) incdec(path string, infoOp func(minfo *minfo)) int {
	c.mu.Lock()
	m := c.counts[path]
	if m == nil {
		m = &minfo{}
		c.counts[path] = m
	}
	// if we are checking this path for the first time check to make sure
	// if it was already mounted on the system and make sure we have a correct ref
	// count if it is mounted as it is in use.
	if !m.check {
		m.check = true
		if c.isMounted(path) {
			m.count++
		}
	}
	infoOp(m)
	count := m.count
	if count <= 0 {
		delete(c.counts, path)
	}
	c.mu.Unlock()
	return count
}
