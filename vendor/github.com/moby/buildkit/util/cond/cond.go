package cond

import (
	"sync"
)

// NewStatefulCond returns a stateful version of sync.Cond . This cond will
// never block on `Wait()` if `Signal()` has been called after the `Wait()` last
// returned. This is useful for avoiding to take a lock on `cond.Locker` for
// signalling.
func NewStatefulCond(l sync.Locker) *StatefulCond {
	sc := &StatefulCond{main: l}
	sc.c = sync.NewCond(&sc.mu)
	return sc
}

type StatefulCond struct {
	main      sync.Locker
	mu        sync.Mutex
	c         *sync.Cond
	signalled bool
}

func (s *StatefulCond) Wait() {
	s.main.Unlock()
	s.mu.Lock()
	if !s.signalled {
		s.c.Wait()
	}
	s.signalled = false
	s.mu.Unlock()
	s.main.Lock()
}

func (s *StatefulCond) Signal() {
	s.mu.Lock()
	s.signalled = true
	s.c.Signal()
	s.mu.Unlock()
}
