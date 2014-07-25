package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/pkg/units"
)

type State struct {
	sync.RWMutex
	Running    bool
	Paused     bool
	Pid        int
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	waitChan   chan struct{}
}

func NewState() *State {
	return &State{
		waitChan: make(chan struct{}),
	}
}

// String returns a human-readable description of the state
func (s *State) String() string {
	s.RLock()
	defer s.RUnlock()

	if s.Running {
		if s.Paused {
			return fmt.Sprintf("Up %s (Paused)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
		}
		return fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
	}
	if s.FinishedAt.IsZero() {
		return ""
	}
	return fmt.Sprintf("Exited (%d) %s ago", s.ExitCode, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
}

func wait(waitChan <-chan struct{}, timeout time.Duration) error {
	if timeout < 0 {
		<-waitChan
		return nil
	}
	select {
	case <-time.After(timeout):
		return fmt.Errorf("Timed out: %v", timeout)
	case <-waitChan:
		return nil
	}
}

// WaitRunning waits until state is running. If state already running it returns
// immediatly. If you want wait forever you must supply negative timeout.
// Returns pid, that was passed to SetRunning
func (s *State) WaitRunning(timeout time.Duration) (int, error) {
	s.RLock()
	if s.IsRunning() {
		pid := s.Pid
		s.RUnlock()
		return pid, nil
	}
	waitChan := s.waitChan
	s.RUnlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	return s.GetPid(), nil
}

// WaitStop waits until state is stopped. If state already stopped it returns
// immediatly. If you want wait forever you must supply negative timeout.
// Returns exit code, that was passed to SetStopped
func (s *State) WaitStop(timeout time.Duration) (int, error) {
	s.RLock()
	if !s.Running {
		exitCode := s.ExitCode
		s.RUnlock()
		return exitCode, nil
	}
	waitChan := s.waitChan
	s.RUnlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	return s.GetExitCode(), nil
}

func (s *State) IsRunning() bool {
	s.RLock()
	res := s.Running
	s.RUnlock()
	return res
}

func (s *State) GetPid() int {
	s.RLock()
	res := s.Pid
	s.RUnlock()
	return res
}

func (s *State) GetExitCode() int {
	s.RLock()
	res := s.ExitCode
	s.RUnlock()
	return res
}

func (s *State) SetRunning(pid int) {
	s.Lock()
	if !s.Running {
		s.Running = true
		s.Paused = false
		s.ExitCode = 0
		s.Pid = pid
		s.StartedAt = time.Now().UTC()
		close(s.waitChan) // fire waiters for start
		s.waitChan = make(chan struct{})
	}
	s.Unlock()
}

func (s *State) SetStopped(exitCode int) {
	s.Lock()
	if s.Running {
		s.Running = false
		s.Pid = 0
		s.FinishedAt = time.Now().UTC()
		s.ExitCode = exitCode
		close(s.waitChan) // fire waiters for stop
		s.waitChan = make(chan struct{})
	}
	s.Unlock()
}

func (s *State) SetPaused() {
	s.Lock()
	s.Paused = true
	s.Unlock()
}

func (s *State) SetUnpaused() {
	s.Lock()
	s.Paused = false
	s.Unlock()
}

func (s *State) IsPaused() bool {
	s.RLock()
	res := s.Paused
	s.RUnlock()
	return res
}
