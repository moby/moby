// Package lamport provides a Lamport logical clock.
package lamport

import "sync/atomic"

// Time represents a logical timestamp in a Lamport clock.
type Time uint64

// Clock is a concurrency-safe Lamport logical clock.
//
// Clock values are uint64 and have finite range. Overflow is not handled
// specially; callers are expected not to exhaust the clock's value space.
type Clock struct {
	time atomic.Uint64
}

// Time returns the current clock value.
func (c *Clock) Time() Time {
	return Time(c.time.Load())
}

// Increment advances the clock by one and returns its new value.
func (c *Clock) Increment() Time {
	return Time(c.time.Add(1))
}

// Witness advances the clock to one greater than other if other is greater
// than or equal to the current clock value.
func (c *Clock) Witness(other Time) {
	o := uint64(other)
	for {
		current := c.time.Load()
		if current > o {
			return
		}
		if c.time.CompareAndSwap(current, o+1) {
			return
		}
	}
}
