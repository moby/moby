package container // import "github.com/docker/docker/container"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
	units "github.com/docker/go-units"
)

// State holds the current container state, and has methods to get and
// set the state. Container has an embed, which allows all of the
// functions defined against State to run against Container.
type State struct {
	sync.Mutex
	// Note that `Running` and `Paused` are not mutually exclusive:
	// When pausing a container (on Linux), the freezer cgroup is used to suspend
	// all processes in the container. Freezing the process requires the process to
	// be running. As a result, paused containers are both `Running` _and_ `Paused`.
	Running           bool
	Paused            bool
	Restarting        bool
	OOMKilled         bool
	RemovalInProgress bool // Not need for this to be persistent on disk.
	Dead              bool
	Pid               int
	ExitCodeValue     int    `json:"ExitCode"`
	ErrorMsg          string `json:"Error"` // contains last known error during container start, stop, or remove
	StartedAt         time.Time
	FinishedAt        time.Time
	Health            *Health
	Removed           bool `json:"-"`

	stopWaiters       []chan<- StateStatus
	removeOnlyWaiters []chan<- StateStatus

	// The libcontainerd reference fields are unexported to force consumers
	// to access them through the getter methods with multi-valued returns
	// so that they can't forget to nil-check: the code won't compile unless
	// the nil-check result is explicitly consumed or discarded.

	ctr  libcontainerdtypes.Container
	task libcontainerdtypes.Task
}

// StateStatus is used to return container wait results.
// Implements exec.ExitCode interface.
// This type is needed as State include a sync.Mutex field which make
// copying it unsafe.
type StateStatus struct {
	exitCode int
	err      error
}

// ExitCode returns current exitcode for the state.
func (s StateStatus) ExitCode() int {
	return s.exitCode
}

// Err returns current error for the state. Returns nil if the container had
// exited on its own.
func (s StateStatus) Err() error {
	return s.err
}

// NewState creates a default state object.
func NewState() *State {
	return &State{}
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

// WaitCondition is an enum type for different states to wait for.
type WaitCondition int

// Possible WaitCondition Values.
//
// WaitConditionNotRunning (default) is used to wait for any of the non-running
// states: "created", "exited", "dead", "removing", or "removed".
//
// WaitConditionNextExit is used to wait for the next time the state changes
// to a non-running state. If the state is currently "created" or "exited",
// this would cause Wait() to block until either the container runs and exits
// or is removed.
//
// WaitConditionRemoved is used to wait for the container to be removed.
const (
	WaitConditionNotRunning WaitCondition = iota
	WaitConditionNextExit
	WaitConditionRemoved
)

// Wait waits until the container is in a certain state indicated by the given
// condition. A context must be used for cancelling the request, controlling
// timeouts, and avoiding goroutine leaks. Wait must be called without holding
// the state lock. Returns a channel from which the caller will receive the
// result. If the container exited on its own, the result's Err() method will
// be nil and its ExitCode() method will return the container's exit code,
// otherwise, the results Err() method will return an error indicating why the
// wait operation failed.
func (s *State) Wait(ctx context.Context, condition WaitCondition) <-chan StateStatus {
	s.Lock()
	defer s.Unlock()

	// Buffer so we can put status and finish even nobody receives it.
	resultC := make(chan StateStatus, 1)

	if s.conditionAlreadyMet(condition) {
		resultC <- StateStatus{
			exitCode: s.ExitCode(),
			err:      s.Err(),
		}

		return resultC
	}

	waitC := make(chan StateStatus, 1)

	// Removal wakes up both removeOnlyWaiters and stopWaiters
	// Container could be removed while still in "created" state
	// in which case it is never actually stopped
	if condition == WaitConditionRemoved {
		s.removeOnlyWaiters = append(s.removeOnlyWaiters, waitC)
	} else {
		s.stopWaiters = append(s.stopWaiters, waitC)
	}

	go func() {
		select {
		case <-ctx.Done():
			// Context timeout or cancellation.
			resultC <- StateStatus{
				exitCode: -1,
				err:      ctx.Err(),
			}
			return
		case status := <-waitC:
			resultC <- status
		}
	}()

	return resultC
}

func (s *State) conditionAlreadyMet(condition WaitCondition) bool {
	switch condition {
	case WaitConditionNotRunning:
		return !s.Running
	case WaitConditionRemoved:
		return s.Removed
	}

	return false
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
func (s *State) SetRunning(ctr libcontainerdtypes.Container, tsk libcontainerdtypes.Task, initial bool) {
	s.ErrorMsg = ""
	s.Paused = false
	s.Running = true
	s.Restarting = false
	if initial {
		s.Paused = false
	}
	s.ExitCodeValue = 0
	s.ctr = ctr
	s.task = tsk
	if tsk != nil {
		s.Pid = int(tsk.Pid())
	} else {
		s.Pid = 0
	}
	s.OOMKilled = false
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
	if exitStatus.ExitedAt.IsZero() {
		s.FinishedAt = time.Now().UTC()
	} else {
		s.FinishedAt = exitStatus.ExitedAt
	}
	s.ExitCodeValue = exitStatus.ExitCode

	s.notifyAndClear(&s.stopWaiters)
}

// SetRestarting sets the container state to "restarting" without locking.
// It also sets the container PID to 0.
func (s *State) SetRestarting(exitStatus *ExitStatus) {
	// we should consider the container running when it is restarting because of
	// all the checks in docker around rm/stop/etc
	s.Running = true
	s.Restarting = true
	s.Paused = false
	s.Pid = 0
	s.FinishedAt = time.Now().UTC()
	s.ExitCodeValue = exitStatus.ExitCode

	s.notifyAndClear(&s.stopWaiters)
}

// SetError sets the container's error state. This is useful when we want to
// know the error that occurred when container transits to another state
// when inspecting it
func (s *State) SetError(err error) {
	s.ErrorMsg = ""
	if err != nil {
		s.ErrorMsg = err.Error()
	}
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

// IsRemovalInProgress returns whether the RemovalInProgress flag is set.
// Used by Container to check whether a container is being removed.
func (s *State) IsRemovalInProgress() bool {
	s.Lock()
	res := s.RemovalInProgress
	s.Unlock()
	return res
}

// IsDead returns whether the Dead flag is set. Used by Container to check whether a container is dead.
func (s *State) IsDead() bool {
	s.Lock()
	res := s.Dead
	s.Unlock()
	return res
}

// SetRemoved assumes this container is already in the "dead" state and notifies all waiters.
func (s *State) SetRemoved() {
	s.SetRemovalError(nil)
}

// SetRemovalError is to be called in case a container remove failed.
// It sets an error and notifies all waiters.
func (s *State) SetRemovalError(err error) {
	s.SetError(err)
	s.Lock()
	s.Removed = true
	s.notifyAndClear(&s.removeOnlyWaiters)
	s.notifyAndClear(&s.stopWaiters)
	s.Unlock()
}

// Err returns an error if there is one.
func (s *State) Err() error {
	if s.ErrorMsg != "" {
		return errors.New(s.ErrorMsg)
	}
	return nil
}

func (s *State) notifyAndClear(waiters *[]chan<- StateStatus) {
	result := StateStatus{
		exitCode: s.ExitCodeValue,
		err:      s.Err(),
	}

	for _, c := range *waiters {
		c <- result
	}
	*waiters = nil
}

// C8dContainer returns a reference to the libcontainerd Container object for
// the container and whether the reference is valid.
//
// The container lock must be held when calling this method.
func (s *State) C8dContainer() (_ libcontainerdtypes.Container, ok bool) {
	return s.ctr, s.ctr != nil
}

// Task returns a reference to the libcontainerd Task object for the container
// and whether the reference is valid.
//
// The container lock must be held when calling this method.
//
// See also: (*Container).GetRunningTask().
func (s *State) Task() (_ libcontainerdtypes.Task, ok bool) {
	return s.task, s.task != nil
}
