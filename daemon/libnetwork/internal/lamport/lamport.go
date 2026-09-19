// Package lamport provides a Lamport logical clock.
package lamport

import "sync/atomic"

// Clock is a concurrency-safe Lamport logical clock.
//
// Clock values are uint64 and have finite range. Overflow is not handled
// specially; callers are expected not to exhaust the clock's value space.
type Clock struct {
	time atomic.Uint64
}

// Time returns the current clock value.
func (c *Clock) Time() uint64 {
	return c.time.Load()
}

// Increment advances the clock by one and returns its new value.
func (c *Clock) Increment() uint64 {
	return c.time.Add(1)
}

// Witness advances the clock to one greater than other if other is greater
// than or equal to the current clock value.
func (c *Clock) Witness(other uint64) {
	for {
		current := c.time.Load()
		if current > other {
			return
		}
		if c.time.CompareAndSwap(current, other+1) {
			return
		}
	}
}
