package state

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/go-units"
)

// Cstate represent a container's running state
type Cstate int64

// Cstate values
const (
	Stopped Cstate = 0
	Running Cstate = (1 << iota)
	Paused
	Restarting
	Dead
)

// State holds the current container state, and has methods to get and
// set the state. Container has an embed, which allows all of the
// functions defined against State to run against Container.
type State struct {
	sync.Mutex
	State             Cstate
	OOMKilled         bool
	RemovalInProgress bool
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

// InState checks whether s is in one of cs state.
// cs can be Cstate(Running|Paused|Restarting)
func (s *State) InState(cs Cstate) bool {
	if s.State&cs != 0 {
		return true
	}

	return false
}

// InStateLocking locks and checks state
func (s *State) InStateLocking(cs Cstate) bool {
	s.Lock()
	defer s.Unlock()
	return s.InState(cs)
}

// String returns a human-readable description of the state
func (s *State) String() string {
	if s.RemovalInProgress {
		return "Removal In Progress"
	}

	switch s.State {
	case Running:
		return fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
	case Paused:
		return fmt.Sprintf("Up %s (Paused)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt)))
	case Restarting:
		return fmt.Sprintf("Restarting (%d) %s ago", s.ExitCode, units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt)))
	case Dead:
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
	switch s.State {
	case Running:
		return "running"
	case Paused:
		return "paused"
	case Restarting:
		return "restarting"
	case Dead:
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

// WaitRunning waits until state is running. If state is already
// running it returns immediately. If you want wait forever you must
// supply negative timeout. Returns pid, that was passed to
// SetRunning.
func (s *State) WaitRunning(timeout time.Duration) (int, error) {
	s.Lock()
	if s.State == Running {
		pid := s.Pid
		s.Unlock()
		return pid, nil
	}
	waitChan := s.waitChan
	s.Unlock()
	if err := wait(waitChan, timeout); err != nil {
		return -1, err
	}
	return s.GetPID(), nil
}

// WaitStop waits until state is stopped. If state already stopped it returns
// immediately. If you want wait forever you must supply negative timeout.
// Returns exit code, that was passed to SetStoppedLocking
func (s *State) WaitStop(timeout time.Duration) (int, error) {
	s.Lock()
	if !s.InState(Running | Paused | Restarting) {
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

// GetPID holds the process id of a container.
func (s *State) GetPID() int {
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

// SetRunning sets the state of the container to "running".
func (s *State) SetRunning(pid int) {
	s.Error = ""
	s.State = Running
	s.ExitCode = 0
	s.Pid = pid
	s.StartedAt = time.Now().UTC()
	close(s.waitChan) // fire waiters for start
	s.waitChan = make(chan struct{})
}

// SetPaused sets the state of the container to "paused"
func (s *State) SetPaused() {
	s.State = Paused
	close(s.waitChan) // fire waiters for start
	s.waitChan = make(chan struct{})
}

// SetStoppedLocking locks the container state is sets it to "stopped".
func (s *State) SetStoppedLocking(exitStatus *execdriver.ExitStatus) {
	s.Lock()
	s.SetStopped(exitStatus)
	s.Unlock()
}

// SetStopped sets the container state to "stopped" without locking.
func (s *State) SetStopped(exitStatus *execdriver.ExitStatus) {
	s.State = Stopped
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.setFromExitStatus(exitStatus)
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// SetRestartingLocking is when docker handles the auto restart of containers when they are
// in the middle of a stop and being restarted again
func (s *State) SetRestartingLocking(exitStatus *execdriver.ExitStatus) {
	s.Lock()
	s.SetRestarting(exitStatus)
	s.Unlock()
}

// SetRestarting sets the container state to "restarting".
// It also sets the container PID to 0.
func (s *State) SetRestarting(exitStatus *execdriver.ExitStatus) {
	// we should consider the container running when it is restarting because of
	// all the checks in docker around rm/stop/etc
	s.State = Restarting
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
	s.Error = err.Error()
}

// SetRemovalInProgressLocking sets the container state as being removed.
// It returns true if the container was already in that state.
func (s *State) SetRemovalInProgressLocking() bool {
	s.Lock()
	defer s.Unlock()
	if s.RemovalInProgress {
		return true
	}
	s.RemovalInProgress = true
	return false
}

// ResetRemovalInProgressLocking make the RemovalInProgress state to false.
func (s *State) ResetRemovalInProgressLocking() {
	s.Lock()
	s.RemovalInProgress = false
	s.Unlock()
}

// SetDeadLocking sets the container state to "dead"
func (s *State) SetDeadLocking() {
	s.Lock()
	defer s.Unlock()
	s.State = Dead
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// SetRawState allows setting any state user specified
func (s *State) SetRawState(cs Cstate) {
	s.State = cs
	close(s.waitChan) // fire waiters for stop
	s.waitChan = make(chan struct{})
}

// SetRawStateLocking allows setting any state user specified
func (s *State) SetRawStateLocking(cs Cstate) {
	s.Lock()
	s.SetRawState(cs)
	s.Unlock()
}

// IsRunning returns whether the running flag is set.
// Used by Container to check whether a container is running
func (s *State) IsRunning() bool {
	return s.InState(Running)
}

// IsRunningLocking locks State and checks whether running flag is set
func (s *State) IsRunningLocking() bool {
	return s.InStateLocking(Running)
}

// IsPaused returns whether the paused flag is set.
// Used by Container to check whether a container is paused
func (s *State) IsPaused() bool {
	return s.InState(Paused)
}

// IsPausedLocking locks State and checks whether paused flag is set
func (s *State) IsPausedLocking() bool {
	return s.InStateLocking(Paused)
}

// IsRestarting returns whether the restarting flag is set.
// Used by Container to check whether a container is restarting
func (s *State) IsRestarting() bool {
	return s.InState(Restarting)
}

// IsRestartingLocking locks State and checks whether restarting flag is set
func (s *State) IsRestartingLocking() bool {
	return s.InStateLocking(Restarting)
}

// IsDead returns whether the dead flag is set.
// Used by Container to check whether a container is dead
func (s *State) IsDead() bool {
	return s.InState(Dead)
}

// IsDeadLocking locks State and checks whether dead flag is set
func (s *State) IsDeadLocking() bool {
	return s.InStateLocking(Dead)
}
