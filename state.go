package docker

import (
	"sync"
)

type State struct {
	Running  bool
	Pid      int
	ExitCode int

	stateChangeLock *sync.Mutex
	stateChangeCond *sync.Cond
}

func newState() *State {
	lock := new(sync.Mutex)
	return &State{
		stateChangeLock: lock,
		stateChangeCond: sync.NewCond(lock),
	}
}

func (s *State) setRunning(pid int) {
	s.Running = true
	s.ExitCode = 0
	s.Pid = pid
	s.broadcast()
}

func (s *State) setStopped(exitCode int) {
	s.Running = false
	s.Pid = 0
	s.ExitCode = exitCode
	s.broadcast()
}

func (s *State) broadcast() {
	s.stateChangeLock.Lock()
	s.stateChangeCond.Broadcast()
	s.stateChangeLock.Unlock()
}

func (s *State) wait() {
	s.stateChangeLock.Lock()
	s.stateChangeCond.Wait()
	s.stateChangeLock.Unlock()
}
