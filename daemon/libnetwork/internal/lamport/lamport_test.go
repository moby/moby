package lamport_test

import (
	"sync"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/lamport"
)

func TestClock(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var c lamport.Clock

		if got := c.Time(); got != 0 {
			t.Fatalf("Time() = %d, want 0", got)
		}
	})

	t.Run("increment", func(t *testing.T) {
		var c lamport.Clock

		if got := c.Increment(); got != 1 {
			t.Fatalf("Increment() = %d, want 1", got)
		}
		if got := c.Increment(); got != 2 {
			t.Fatalf("Increment() = %d, want 2", got)
		}
		if got := c.Time(); got != 2 {
			t.Fatalf("Time() = %d, want 2", got)
		}
	})

	t.Run("witness", func(t *testing.T) {
		var c lamport.Clock

		c.Witness(10)
		if got := c.Time(); got != 11 {
			t.Fatalf("Time() = %d, want 11", got)
		}

		// Older values must not move the clock.
		c.Witness(5)
		if got := c.Time(); got != 11 {
			t.Fatalf("Time() = %d, want 11", got)
		}

		// Witnessing the current value advances past it.
		c.Witness(11)
		if got := c.Time(); got != 12 {
			t.Fatalf("Time() = %d, want 12", got)
		}
	})
}

func TestClockConcurrent(t *testing.T) {
	var c lamport.Clock
	const goroutines = 100

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			c.Increment()
		})
	}

	wg.Wait()

	if got := c.Time(); got != goroutines {
		t.Fatalf("Time() = %d, want %d", got, goroutines)
	}
}
