package refcount

import (
	"sync"
	"sync/atomic"
)

func New() *Counter {
	return &Counter{
		d: make(map[interface{}]*data),
	}
}

// Counter holds reference counts for active objects along with
// additional metadata.
type Counter struct {
	m sync.Mutex
	d map[interface{}]*data
}

type data struct {
	count int64
	value interface{}
}

func (d *data) incr() int64 {
	return atomic.AddInt64(&d.count, 1)
}

func (d *data) decr() int64 {
	return atomic.AddInt64(&d.count, -1)
}

// Count returns the current reference count for an object.
func (c *Counter) Count(k interface{}) int64 {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		return 0
	}
	return d.count
}

// Incr increments the reference count for k.
func (c *Counter) Incr(k interface{}) int64 {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		d = &data{}
		c.d[k] = d
	}
	return d.incr()
}

// Decr decrements the reference count for k.
func (c *Counter) Decr(k interface{}) int64 {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		d = &data{}
		c.d[k] = d
	}
	return d.decr()
}

// Set sets the value v for k.
func (c *Counter) Set(k, v interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		d = &data{}
		c.d[k] = d
	}
	d.value = v
}

// SetIncr sets the value for k and atomically increments the reference count.
func (c *Counter) SetIncr(k, v interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		d = &data{}
		c.d[k] = d
	}
	d.value = v
	d.incr()
}

// Get returns the value for k.  If k does not exist within the counter false
// is returned as the first return result.  If the value does exist the first
// return result is true along with the value.
func (c *Counter) Get(k interface{}) (bool, interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		return false, nil
	}
	return true, d.value
}

// GetIncr increments the count for k and returns the associated value.
// If k does not exist in the counter the first return result is false with a nil value.
// If k does exist then the first return result is true and the value is returned.  Without the
// count being incremented.
func (c *Counter) GetIncr(k interface{}) (bool, interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	d := c.d[k]
	if d == nil {
		return false, nil
	}
	d.incr()
	return true, d.value
}

// Delete removes the specified key from the counter.
func (c *Counter) Delete(k interface{}) {
	delete(c.d, k)
}
