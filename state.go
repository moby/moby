package docker

import (
	"fmt"
	"time"
)

type State struct {
	Running   bool
	Pid       int
	ExitCode  int
	StartedAt time.Time
}

// String returns a human-readable description of the state
func (s *State) String() string {
	if s.Running {
		return fmt.Sprintf("Up %s", HumanDuration(time.Now().Sub(s.StartedAt)))
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
