package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/units"
)

// State holds the current container state, and has methods to get and
// set the state. Container has an embed, which allows all of the
// functions defined against State to run against Container.
type State struct {
	sync.Mutex
	// FIXME: Why do we have both paused and running if a
	// container cannot be paused and running at the same time?
	Running           bool
	Paused            bool
	Restarting        bool
	OOMKilled         bool
	removalInProgress bool // Not need for this to be persistent on disk.
	Dead              bool
	Pid               int
	ExitCode          int
	Error             string // contains last known error when starting the container
	StartedAt         time.Time
	FinishedAt        time.Time
	waitChan          chan struct{}
}

// NewState creates a default state object with a fresh channel for state changes.
func NewState() *State {
	return &State{
		waitChan: make(chan struct{}),
	}
}

// String returns a human-readable description of the state
func (s *State) String() string {
	if s.Running {
		if s.Paused {
			return fmt.Sprintf("Up %s (Paused)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
		}
		if s.Restarting {
			return fmt.Sprintf("Restarting (%d) %s ago", s.ExitCode, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
		}

		return fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
	}

	if s.removalInProgress {
		return "Removal In Progress"
	}

	if s.Dead {
		return "Dead"
	}

	if s.StartedAt.IsZero() {
		return "Created"
	}

	if s.FinishedAt.IsZero() {
		return ""
	}

	return fmt.Sprintf("Exited (%d) %s ago", s.ExitCode, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
}

// StateString returns a single string to describe state
func (s *State) StateString() string {
	if s.Running {
		if s.Paused {
			return "paused"
		}
		if s.Restarting {
			return "restarting"
		}
		return "running"
	}

	if s.Dead {
		return "dead"
	}

	if s.StartedAt.IsZero() {
		return "created"
	}

	return "exited"
}

func isValidStateString(s string) bool {
	if s != "paused" &&
		s != "restarting" &&
		s != "running" &&
		s != "dead" &&
		s != "created" &&
		s != "exited" {
		return false
	}
	return true
}

func wait(waitChan <-chan struct{}, timeout time.Duration) error {
	if timeout < 0 {
		<-waitChan
		return nil
	}
	select {
	case <-time.After(timeout):
		return derr.ErrorCodeTimedOut.WithArgs(timeout)
	case <-waitChan:
		return nil
	}
}

// waitRunning waits until state is running. If state is already
// running it returns immediately. If you want wait forever you must
// supply negative timeout. Returns pid, that was passed to
// setRunning.
func (s *State) waitRunning(timeout time.Duration) (int, error) {
	s.Lock()
	if s.Running {
		pid := s.Pid
		s.Unlock()
		return pid, nil
	}
	waitChan := s.waitChan
	s.Unlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	return s.getPID(), nil
}

// WaitStop waits until state is stopped. If state already stopped it returns
// immediately. If you want wait forever you must supply negative timeout.
// Returns exit code, that was passed to setStoppedLocking
func (s *State) WaitStop(timeout time.Duration) (int, error) {
	s.Lock()
	if !s.Running {
		exitCode := s.ExitCode
		s.Unlock()
		return exitCode, nil
	}
	waitChan := s.waitChan
	s.Unlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	return s.getExitCode(), nil
}

// IsRunning returns whether the running flag is set. Used by Container to check whether a container is running.
func (s *State) IsRunning() bool {
	s.Lock()
	res := s.Running
	s.Unlock()
	return res
}

// GetPID holds the process id of a container.
func (s *State) getPID() int {
	s.Lock()
	res := s.Pid
	s.Unlock()
	return res
}

func (s *State) getExitCode() int {
	s.Lock()
	res := s.ExitCode
	s.Unlock()
	return res
}

func (s *State) setRunning(pid int) {
	s.Error = ""
	s.Running = true
	s.Paused = false
	s.Restarting = false
	s.ExitCode = 0
	s.Pid = pid
	s.StartedAt = time.Now().UTC()
	close(s.waitChan) // fire waiters for start
	s.waitChan = make(chan struct{})
}

func (s *State) setStoppedLocking(exitStatus *execdriver.ExitStatus) {
	s.Lock()
	s.setStopped(exitStatus)
	s.Unlock()
}

func (s *State) setStopped(exitStatus *execdriver.ExitStatus) {
	s.Running = false
	s.Restarting = false
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.ExitCode = exitStatus.ExitCode
	s.OOMKilled = exitStatus.OOMKilled
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// setRestarting is when docker handles the auto restart of containers when they are
// in the middle of a stop and being restarted again
func (s *State) setRestartingLocking(exitStatus *execdriver.ExitStatus) {
	s.Lock()
	s.setRestarting(exitStatus)
	s.Unlock()
}

func (s *State) setRestarting(exitStatus *execdriver.ExitStatus) {
	// we should consider the container running when it is restarting because of
	// all the checks in docker around rm/stop/etc
	s.Running = true
	s.Restarting = true
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.ExitCode = exitStatus.ExitCode
	s.OOMKilled = exitStatus.OOMKilled
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// setError sets the container's error state. This is useful when we want to
// know the error that occurred when container transits to another state
// when inspecting it
func (s *State) setError(err error) {
	s.Error = err.Error()
}

func (s *State) isPaused() bool {
	s.Lock()
	res := s.Paused
	s.Unlock()
	return res
}

func (s *State) setRemovalInProgress() error {
	s.Lock()
	defer s.Unlock()
	if s.removalInProgress {
		return derr.ErrorCodeAlreadyRemoving
	}
	s.removalInProgress = true
	return nil
}

func (s *State) resetRemovalInProgress() {
	s.Lock()
	s.removalInProgress = false
	s.Unlock()
}

func (s *State) setDead() {
	s.Lock()
	s.Dead = true
	s.Unlock()
}
