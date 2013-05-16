package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
	"sync"
	"time"
)

type State struct {
	Running   bool
	Pid       int
	ExitCode  int
	StartedAt time.Time
	l         *sync.Mutex
	Ghost     bool
}

// String returns a human-readable description of the state
func (s *State) String() string {
	if s.Running {
		if s.Ghost {
			return fmt.Sprintf("Ghost")
		}
		return fmt.Sprintf("Up %s", utils.HumanDuration(time.Now().Sub(s.StartedAt)))
	}
	return fmt.Sprintf("Exit %d", s.ExitCode)
}

func (s *State) setRunning(pid int) {
	s.Running = true
	s.ExitCode = 0
	s.Pid = pid
	s.StartedAt = time.Now()
}

func (s *State) setStopped(exitCode int) {
	s.Running = false
	s.Pid = 0
	s.ExitCode = exitCode
}

func (s *State) initLock() {
	s.l = &sync.Mutex{}
}

func (s *State) lock() {
	s.l.Lock()
}

func (s *State) unlock() {
	s.l.Unlock()
}
