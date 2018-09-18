package throttle

import (
	"sync"
	"time"
)

// Throttle wraps a function so that internal function does not get called
// more frequently than the specified duration.
func Throttle(d time.Duration, f func()) func() {
	return throttle(d, f, true)
}

// ThrottleAfter wraps a function so that internal function does not get called
// more frequently than the specified duration. The delay is added after function
// has been called.
func ThrottleAfter(d time.Duration, f func()) func() {
	return throttle(d, f, false)
}

func throttle(d time.Duration, f func(), wait bool) func() {
	var next, running bool
	var mu sync.Mutex
	return func() {
		mu.Lock()
		defer mu.Unlock()

		next = true
		if !running {
			running = true
			go func() {
				for {
					mu.Lock()
					if next == false {
						running = false
						mu.Unlock()
						return
					}
					if !wait {
						next = false
					}
					mu.Unlock()

					if wait {
						time.Sleep(d)
						mu.Lock()
						next = false
						mu.Unlock()
						f()
					} else {
						f()
						time.Sleep(d)
					}
				}
			}()
		}
	}
}
