package container // import "github.com/docker/docker/container"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	units "github.com/docker/go-units"
)

// State holds the current container state, and has methods to get and
// set the state. Container has an embed, which allows all of the
// functions defined against State to run against Container.
type State struct {
	// Note that `Running` and `Paused` are not mutually exclusive:
	// When pausing a container (on Linux), the freezer cgroup is used to suspend
	// all processes in the container. Freezing the process requires the process to
	// be running. As a result, paused containers are both `Running` _and_ `Paused`.
	Running           sbool
	Paused            sbool
	Restarting        sbool
	OOMKilled         sbool
	RemovalInProgress sbool
	Dead              sbool
	Pid               int64
	ExitCodeValue     int64   `json:"ExitCode"`
	ErrorMsg          sstring `json:"Error"`
	StartedAt         stime
	FinishedAt        stime
	Health            *Health

	waitStop   chan struct{}
	waitRemove chan struct{}

	mu sync.Mutex
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

// NewState creates a default state object with a fresh channel for state changes.
func NewState() *State {
	return &State{
		waitStop:   make(chan struct{}),
		waitRemove: make(chan struct{}),
	}
}

// String returns a human-readable description of the state
func (s *State) String() string {
	if s.IsRunning() {
		if s.IsPaused() {
			return fmt.Sprintf("Up %s (Paused)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt.Get())))
		}
		if s.IsRestarting() {
			return fmt.Sprintf("Restarting (%d) %s ago", s.ExitCode(), units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt.Get())))
		}

		if h := s.Health; h != nil {
			return fmt.Sprintf("Up %s (%s)", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt.Get())), h.String())
		}

		return fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(s.StartedAt.Get())))
	}

	if s.IsRemovalInProgress() {
		return "Removal In Progress"
	}

	if s.IsDead() {
		return "Dead"
	}

	if s.StartedAt.Get().IsZero() {
		return "Created"
	}

	if s.FinishedAt.Get().IsZero() {
		return ""
	}

	return fmt.Sprintf("Exited (%d) %s ago", s.ExitCode(), units.HumanDuration(time.Now().UTC().Sub(s.FinishedAt.Get())))
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
	if s.IsRunning() {
		if s.IsPaused() {
			return "paused"
		}
		if s.IsRestarting() {
			return "restarting"
		}
		return "running"
	}

	if s.IsRemovalInProgress() {
		return "removing"
	}

	if s.IsDead() {
		return "dead"
	}

	if s.StartedAt.Get().IsZero() {
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
	if condition == WaitConditionNotRunning && !s.IsRunning() {
		// Buffer so we can put it in the channel now.
		resultC := make(chan StateStatus, 1)

		// Send the current status.
		resultC <- StateStatus{
			exitCode: s.ExitCode(),
			err:      s.Err(),
		}

		return resultC
	}

	// If we are waiting only for removal, the waitStop channel should
	// remain nil and block forever.
	s.mu.Lock()
	var waitStop chan struct{}
	if condition < WaitConditionRemoved {
		waitStop = s.waitStop
	}

	// Always wait for removal, just in case the container gets removed
	// while it is still in a "created" state, in which case it is never
	// actually stopped.
	waitRemove := s.waitRemove
	s.mu.Unlock()

	resultC := make(chan StateStatus)

	go func() {
		select {
		case <-ctx.Done():
			// Context timeout or cancellation.
			resultC <- StateStatus{
				exitCode: -1,
				err:      ctx.Err(),
			}
			return
		case <-waitStop:
		case <-waitRemove:
		}

		result := StateStatus{
			exitCode: s.ExitCode(),
			err:      s.Err(),
		}

		resultC <- result
	}()

	return resultC
}

// IsRunning returns whether the running flag is set. Used by Container to check whether a container is running.
func (s *State) IsRunning() bool {
	return atomic.LoadInt64(&s.Running.v) == 1
}

// GetPID holds the process id of a container.
func (s *State) GetPID() int {
	return int(atomic.LoadInt64(&s.Pid))
}

// GetPID holds the process id of a container.
func (s *State) SetPID(pid int) {
	atomic.StoreInt64(&s.Pid, int64(pid))
}

// ExitCode returns current exitcode for the state.
func (s *State) ExitCode() int {
	return int(atomic.LoadInt64(&s.ExitCodeValue))
}

// SetExitCode sets current exitcode for the state.
func (s *State) SetExitCode(ec int) {
	atomic.StoreInt64(&s.ExitCodeValue, int64(ec))
}

// SetRunning sets the state of the container to "running".
func (s *State) SetRunning(pid int, initial bool) {
	s.SetError(nil)
	s.Running.Set(true)
	s.SetPaused(false)
	s.Restarting.Set(false)
	s.SetExitCode(0)
	s.SetPID(pid)
	if initial {
		s.StartedAt.Set(time.Now().UTC())
	}
}

// SetStopped sets the container state to "stopped" without locking.
func (s *State) SetStopped(exitStatus *ExitStatus) {
	s.Running.Set(false)
	s.SetPaused(false)
	s.Restarting.Set(false)
	s.SetPID(0)
	if exitStatus.ExitedAt.IsZero() {
		s.FinishedAt.Set(time.Now().UTC())
	} else {
		s.FinishedAt.Set(exitStatus.ExitedAt)
	}
	s.SetExitCode(exitStatus.ExitCode)
	s.SetOOMKilled(exitStatus.OOMKilled)
	close(s.waitStop) // fire waiters for stop
	s.mu.Lock()
	s.waitStop = make(chan struct{})
	s.mu.Unlock()
}

// SetRestarting sets the container state to "restarting" without locking.
// It also sets the container PID to 0.
func (s *State) SetRestarting(exitStatus *ExitStatus) {
	// we should consider the container running when it is restarting because of
	// all the checks in docker around rm/stop/etc
	s.Running.Set(true)
	s.SetPaused(false)
	s.Restarting.Set(true)
	s.SetPID(0)
	s.FinishedAt.Set(time.Now().UTC())
	s.SetExitCode(exitStatus.ExitCode)
	s.SetOOMKilled(exitStatus.OOMKilled)
	close(s.waitStop) // fire waiters for stop
	s.mu.Lock()
	s.waitStop = make(chan struct{})
	s.mu.Unlock()
}

// SetError sets the container's error state. This is useful when we want to
// know the error that occurred when container transits to another state
// when inspecting it
func (s *State) SetError(err error) {
	if err != nil {
		s.ErrorMsg.Set(err.Error())
	} else {
		s.ErrorMsg.Set("")
	}
}

// IsPaused returns whether the container is paused or not.
func (s *State) IsPaused() bool {
	return s.Paused.Get()
}

// SetPaused sets the paused value
func (s *State) SetPaused(b bool) {
	s.Paused.Set(b)
}

// IsOOMKilled returns whether the container is oom killed or not.
func (s *State) IsOOMKilled() bool {
	return s.OOMKilled.Get()
}

// SetOOMKilled sets the paused value
func (s *State) SetOOMKilled(b bool) {
	s.OOMKilled.Set(b)
}

// IsRestarting returns whether the container is restarting or not.
func (s *State) IsRestarting() bool {
	return s.Restarting.Get()
}

// SetRemovalInProgress sets the container state as being removed.
// It returns true if the container was already in that state.
func (s *State) SetRemovalInProgress(b bool) {
	s.RemovalInProgress.Set(b)
}

// IsRemovalInProgress returns whether the RemovalInProgress flag is set.
// Used by Container to check whether a container is being removed.
func (s *State) IsRemovalInProgress() bool {
	return s.RemovalInProgress.Get()
}

// SetDead sets the container state to "dead"
func (s *State) SetDead() {
	s.Dead.Set(true)
}

// IsDead returns whether the Dead flag is set. Used by Container to check whether a container is dead.
func (s *State) IsDead() bool {
	return s.Dead.Get()
}

// SetRemoved assumes this container is already in the "dead" state and
// closes the internal waitRemove channel to unblock callers waiting for a
// container to be removed.
func (s *State) SetRemoved() {
	s.SetRemovalError(nil)
}

// SetRemovalError is to be called in case a container remove failed.
// It sets an error and closes the internal waitRemove channel to unblock
// callers waiting for the container to be removed.
func (s *State) SetRemovalError(err error) {
	s.SetError(err)
	close(s.waitRemove) // Unblock those waiting on remove.
	s.mu.Lock()
	// Recreate the channel so next ContainerWait will work
	s.waitRemove = make(chan struct{})
	s.mu.Unlock()
}

// Err returns an error if there is one.
func (s *State) Err() error {
	msg := s.ErrorMsg.Get()
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}
