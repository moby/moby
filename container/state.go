package container

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-units"
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
	RemovalInProgress bool // Not need for this to be persistent on disk.
	Dead              bool
	Pid               int
	ExitCodeValue     int    `json:"ExitCode"`
	ErrorMsg          string `json:"Error"` // contains last known error when starting the container
	StartedAt         time.Time
	FinishedAt        time.Time
	waitChan          chan struct{}
	Health            *Health
}

// StateStatus is used to return an error type implementing both
// exec.ExitCode and error.
// This type is needed as State include a sync.Mutex field which make
// copying it unsafe.
type StateStatus struct {
	exitCode int
	error    string
}

func newStateStatus(ec int, err string) *StateStatus {
	return &StateStatus{
		exitCode: ec,
		error:    err,
	}
}

// ExitCode returns current exitcode for the state.
func (ss *StateStatus) ExitCode() int {
	return ss.exitCode
}

// Error returns current error for the state.
func (ss *StateStatus) Error() string {
	return ss.error
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
			return fmt.Sprintf("Restarting (%d) %s ago", s.ExitCodeValue, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
		}

		if h := s.Health; h != nil {
			return fmt.Sprintf("Up %s (%s)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)), h.String())
		}

		return fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
	}

	if s.RemovalInProgress {
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

	return fmt.Sprintf("Exited (%d) %s ago", s.ExitCodeValue, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
}

// HealthString returns a single string to describe health status.
func (s *State) HealthString() string {
	if s.Health == nil {
		return types.NoHealthcheck
	}

	return s.Health.String()
}

// IsValidHealthString checks if the provided string is a valid container health status or not.
func IsValidHealthString(s string) bool {
	return s == types.Starting ||
		s == types.Healthy ||
		s == types.Unhealthy ||
		s == types.NoHealthcheck
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

	if s.RemovalInProgress {
		return "removing"
	}

	if s.Dead {
		return "dead"
	}

	if s.StartedAt.IsZero() {
		return "created"
	}

	return "exited"
}

// IsValidStateString checks if the provided string is a valid container state or not.
func IsValidStateString(s string) bool {
	if s != "paused" &&
		s != "restarting" &&
		s != "removing" &&
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
		return fmt.Errorf("Timed out: %v", timeout)
	case <-waitChan:
		return nil
	}
}

// WaitStop waits until state is stopped. If state already stopped it returns
// immediately. If you want wait forever you must supply negative timeout.
// Returns exit code, that was passed to SetStopped
func (s *State) WaitStop(timeout time.Duration) (int, error) {
	s.Lock()
	if !s.Running {
		exitCode := s.ExitCodeValue
		s.Unlock()
		return exitCode, nil
	}
	waitChan := s.waitChan
	s.Unlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	s.Lock()
	defer s.Unlock()
	return s.ExitCode(), nil
}

// WaitWithContext waits for the container to stop. Optional context can be
// passed for canceling the request.
func (s *State) WaitWithContext(ctx context.Context) error {
	// todo(tonistiigi): make other wait functions use this
	s.Lock()
	if !s.Running {
		state := newStateStatus(s.ExitCode(), s.Error())
		defer s.Unlock()
		if state.ExitCode() == 0 {
			return nil
		}
		return state
	}
	waitChan := s.waitChan
	s.Unlock()
	select {
	case <-waitChan:
		s.Lock()
		state := newStateStatus(s.ExitCode(), s.Error())
		s.Unlock()
		if state.ExitCode() == 0 {
			return nil
		}
		return state
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsRunning returns whether the running flag is set. Used by Container to check whether a container is running.
func (s *State) IsRunning() bool {
	s.Lock()
	res := s.Running
	s.Unlock()
	return res
}

// GetPID holds the process id of a container.
func (s *State) GetPID() int {
	s.Lock()
	res := s.Pid
	s.Unlock()
	return res
}

// ExitCode returns current exitcode for the state. Take lock before if state
// may be shared.
func (s *State) ExitCode() int {
	return s.ExitCodeValue
}

// SetExitCode sets current exitcode for the state. Take lock before if state
// may be shared.
func (s *State) SetExitCode(ec int) {
	s.ExitCodeValue = ec
}

// SetRunning sets the state of the container to "running".
func (s *State) SetRunning(pid int, initial bool) {
	s.ErrorMsg = ""
	s.Running = true
	s.Restarting = false
	s.ExitCodeValue = 0
	s.Pid = pid
	if initial {
		s.StartedAt = time.Now().UTC()
	}
}

// SetStopped sets the container state to "stopped" without locking.
func (s *State) SetStopped(exitStatus *ExitStatus) {
	s.Running = false
	s.Paused = false
	s.Restarting = false
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.setFromExitStatus(exitStatus)
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// SetRestarting sets the container state to "restarting" without locking.
// It also sets the container PID to 0.
func (s *State) SetRestarting(exitStatus *ExitStatus) {
	// we should consider the container running when it is restarting because of
	// all the checks in docker around rm/stop/etc
	s.Running = true
	s.Restarting = true
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.setFromExitStatus(exitStatus)
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// SetError sets the container's error state. This is useful when we want to
// know the error that occurred when container transits to another state
// when inspecting it
func (s *State) SetError(err error) {
	s.ErrorMsg = err.Error()
}

// IsPaused returns whether the container is paused or not.
func (s *State) IsPaused() bool {
	s.Lock()
	res := s.Paused
	s.Unlock()
	return res
}

// IsRestarting returns whether the container is restarting or not.
func (s *State) IsRestarting() bool {
	s.Lock()
	res := s.Restarting
	s.Unlock()
	return res
}

// SetRemovalInProgress sets the container state as being removed.
// It returns true if the container was already in that state.
func (s *State) SetRemovalInProgress() bool {
	s.Lock()
	defer s.Unlock()
	if s.RemovalInProgress {
		return true
	}
	s.RemovalInProgress = true
	return false
}

// ResetRemovalInProgress makes the RemovalInProgress state to false.
func (s *State) ResetRemovalInProgress() {
	s.Lock()
	s.RemovalInProgress = false
	s.Unlock()
}

// SetDead sets the container state to "dead"
func (s *State) SetDead() {
	s.Lock()
	s.Dead = true
	s.Unlock()
}

// Error returns current error for the state.
func (s *State) Error() string {
	return s.ErrorMsg
}
