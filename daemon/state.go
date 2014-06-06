package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/dotcloud/docker/pkg/units"
)

type State struct {
	sync.RWMutex
	Running    bool
	Paused     bool
	Pid        int
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
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

func (s *State) IsRunning() bool {
	s.RLock()
	defer s.RUnlock()

	return s.Running
}

func (s *State) GetExitCode() int {
	s.RLock()
	defer s.RUnlock()

	return s.ExitCode
}

func (s *State) SetRunning(pid int) {
	s.Lock()
	defer s.Unlock()

	s.Running = true
	s.Paused = false
	s.ExitCode = 0
	s.Pid = pid
	s.StartedAt = time.Now().UTC()
}

func (s *State) SetStopped(exitCode int) {
	s.Lock()
	defer s.Unlock()

	s.Running = false
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.ExitCode = exitCode
}

func (s *State) SetPaused() {
	s.Lock()
	defer s.Unlock()
	s.Paused = true
}

func (s *State) SetUnpaused() {
	s.Lock()
	defer s.Unlock()
	s.Paused = false
}

func (s *State) IsPaused() bool {
	s.RLock()
	defer s.RUnlock()

	return s.Paused
}
