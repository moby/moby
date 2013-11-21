package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"sync"
	"time"
)

type State struct {
	sync.RWMutex
	running    bool
	pid        int
	exitCode   int
	startedAt  time.Time
	finishedAt time.Time
	ghost      bool
}

// String returns a human-readable description of the state
func (s *State) String() string {
	s.RLock()
	defer s.RUnlock()

	if s.running {
		if s.ghost {
			return fmt.Sprintf("Ghost")
		}
		return fmt.Sprintf("Up %s", utils.HumanDuration(time.Now().Sub(s.startedAt)))
	}
	return fmt.Sprintf("Exit %d", s.exitCode)
}

func (s *State) IsRunning() bool {
	s.RLock()
	defer s.RUnlock()

	return s.running
}

func (s *State) IsGhost() bool {
	s.RLock()
	defer s.RUnlock()

	return s.ghost
}

func (s *State) GetExitCode() int {
	s.RLock()
	defer s.RUnlock()

	return s.exitCode
}

func (s *State) SetGhost(val bool) {
	s.Lock()
	defer s.Unlock()

	s.ghost = val
}

func (s *State) SetRunning(pid int) {
	s.Lock()
	defer s.Unlock()

	s.running = true
	s.ghost = false
	s.exitCode = 0
	s.pid = pid
	s.startedAt = time.Now().UTC()
}

func (s *State) SetStopped(exitCode int) {
	s.Lock()
	defer s.Unlock()

	s.running = false
	s.pid = 0
	s.finishedAt = time.Now().UTC()
	s.exitCode = exitCode
}
