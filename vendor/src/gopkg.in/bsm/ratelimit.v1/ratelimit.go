/*
Simple, thread-safe Go rate-limiter.
Inspired by Antti Huima's algorithm on http://stackoverflow.com/a/668327

Example:

  // Create a new rate-limiter, allowing up-to 10 calls
  // per second
  rl := ratelimit.New(10, time.Second)

  for i:=0; i<20; i++ {
    if rl.Limit() {
      fmt.Println("DOH! Over limit!")
    } else {
      fmt.Println("OK")
    }
  }
*/
package ratelimit

import (
	"sync/atomic"
	"time"
)

// RateLimit instances are thread-safe.
type RateLimiter struct {
	rate, allowance, max, unit, lastCheck uint64
}

// New creates a new rate limiter instance
func New(rate int, per time.Duration) *RateLimiter {
	nano := uint64(per)
	if nano < 1 {
		nano = uint64(time.Second)
	}
	if rate < 1 {
		rate = 1
	}

	return &RateLimiter{
		rate:      uint64(rate),        // store the rate
		allowance: uint64(rate) * nano, // set our allowance to max in the beginning
		max:       uint64(rate) * nano, // remember our maximum allowance
		unit:      nano,                // remember our unit size

		lastCheck: unixNano(),
	}
}

// Limit returns true if rate was exceeded
func (rl *RateLimiter) Limit() bool {
	// Calculate the number of ns that have passed since our last call
	now := unixNano()
	passed := now - atomic.SwapUint64(&rl.lastCheck, now)

	// Add them to our allowance
	current := atomic.AddUint64(&rl.allowance, passed*rl.rate)

	// Ensure our allowance is not over maximum
	if current > rl.max {
		atomic.AddUint64(&rl.allowance, rl.max-current)
		current = rl.max
	}

	// If our allowance is less than one unit, rate-limit!
	if current < rl.unit {
		return true
	}

	// Not limited, subtract a unit
	atomic.AddUint64(&rl.allowance, -rl.unit)
	return false
}

// Undo reverts the last Limit() call, returning consumed allowance
func (rl *RateLimiter) Undo() {
	current := atomic.AddUint64(&rl.allowance, rl.unit)

	// Ensure our allowance is not over maximum
	if current > rl.max {
		atomic.AddUint64(&rl.allowance, rl.max-current)
	}
}

// now as unix nanoseconds
func unixNano() uint64 {
	return uint64(time.Now().UnixNano())
}
