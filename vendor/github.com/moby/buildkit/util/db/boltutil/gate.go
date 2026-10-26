package boltutil

import (
	"context"
	"sync"
)

// gate coordinates transactions across replacement of the underlying bbolt handle.
// bbolt's transaction locks protect one handle, not the wrapper's handle swap.
// A pause blocks new entrants and drains active callbacks, then keeps transactions
// paused until copying and replacement finish. No callback may retain the old handle.
//
// Draining must be cancellable: an active callback can depend on a nested
// transaction blocked by the pause. The caller's drain timeout breaks that wait
// by reopening the gate and skipping compaction. RWMutex.Lock cannot be canceled,
// while polling TryLock can starve under continuous transaction activity.
//
// mu protects all gate state. A non-nil resume blocks entry until unpause closes
// it; idle lets the drainer wait for active to reach zero without holding mu.
type gate struct {
	mu     sync.Mutex
	active int
	resume chan struct{}
	idle   chan struct{}
	closed bool
}

func (g *gate) enter() bool {
	for {
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			return false
		}
		resume := g.resume
		if resume == nil {
			g.active++
			g.mu.Unlock()
			return true
		}
		g.mu.Unlock()
		<-resume
	}
}

// close rejects new transactions, including nested ones, before waiting for
// existing callbacks to finish. It must not run concurrently with a pause.
func (g *gate) close() {
	g.mu.Lock()
	g.closed = true
	if g.active == 0 {
		g.mu.Unlock()
		return
	}
	idle := make(chan struct{})
	g.idle = idle
	g.mu.Unlock()
	<-idle
}

func (g *gate) exit() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.active == 0 && g.idle != nil {
		close(g.idle)
		g.idle = nil
	}
}

// The caller serializes pauses and pairs a successful pause with unpause.
func (g *gate) pause(ctx context.Context) bool {
	g.mu.Lock()
	g.resume = make(chan struct{})
	idle := make(chan struct{})
	if g.active == 0 {
		close(idle)
	} else {
		g.idle = idle
	}
	g.mu.Unlock()
	select {
	case <-idle:
		if context.Cause(ctx) == nil {
			return true
		}
	case <-ctx.Done():
	}
	g.unpause()
	return false
}

func (g *gate) unpause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.idle = nil
	close(g.resume)
	g.resume = nil
}
