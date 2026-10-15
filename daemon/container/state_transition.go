package container

import (
	"errors"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/moby/moby/v2/errdefs"
)

// StateSnapshot is a copy of a container's state.
type StateSnapshot struct {
	Status     container.ContainerState
	PID        int
	ExitCode   int
	Error      string
	OOMKilled  bool
	StartedAt  time.Time
	FinishedAt time.Time
	// Health is empty if the container has no healthcheck.
	Health container.HealthStatus
}

// StateChange is a container's state before and after a change.
type StateChange struct {
	Previous StateSnapshot
	Current  StateSnapshot
}

// Transition describes a change of a container's status.
type Transition struct {
	// To is the status to change to: [container.StateRunning],
	// [container.StatePaused], [container.StateRestarting],
	// [container.StateExited], [container.StateRemoving], or
	// [container.StateDead].
	To container.ContainerState
	// ExitCode and ExitedAt describe the exit for [container.StateRestarting]
	// and [container.StateExited]; a zero ExitedAt means now.
	ExitCode int
	ExitedAt time.Time
	// Error, if not nil, replaces the recorded error for
	// [container.StateRestarting] and [container.StateExited].
	Error error
	// Container and Task are the started container and task for
	// [container.StateRunning].
	Container libcontainerdtypes.Container
	Task      libcontainerdtypes.Task
	// StartedAt is when the container started for [container.StateRunning];
	// zero keeps the previous start time, for containers that the daemon did
	// not start.
	StartedAt time.Time
}

// Transition changes the container's status and returns the change.
// Moving a paused container to [container.StateRunning] unpauses it.
// It returns a Conflict error if the container is not running when pausing,
// or is already being removed.
// The caller must hold the state lock.
func (s *State) Transition(t Transition) (StateChange, error) {
	var apply func()
	switch t.To {
	case container.StateRunning:
		if s.Paused {
			apply = func() { s.Paused = false }
		} else {
			apply = func() { s.start(t) }
		}
	case container.StatePaused:
		if s.State() != container.StateRunning {
			return StateChange{}, errdefs.Conflict(errors.New("container is not running"))
		}
		apply = func() { s.Paused = true }
	case container.StateRestarting:
		apply = func() { s.exit(t, true) }
	case container.StateExited:
		apply = func() { s.exit(t, false) }
	case container.StateRemoving:
		if s.RemovalInProgress {
			return StateChange{}, errdefs.Conflict(errors.New("removal is already in progress"))
		}
		apply = func() { s.RemovalInProgress = true }
	case container.StateDead:
		apply = func() { s.Dead = true }
	default:
		return StateChange{}, errdefs.InvalidParameter(fmt.Errorf("invalid container status transition to %q", t.To))
	}
	return s.change(apply), nil
}

func (s *State) start(t Transition) {
	s.ErrorMsg = ""
	s.Paused = false
	s.Running = true
	s.Restarting = false
	s.ExitCode = 0
	s.ctr = t.Container
	s.task = t.Task
	s.Pid = 0
	if t.Task != nil {
		s.Pid = int(t.Task.Pid())
	}
	s.OOMKilled = false
	if !t.StartedAt.IsZero() {
		s.StartedAt = t.StartedAt.UTC()
	}
}

// exit records an exit. A restarting container is still considered running,
// because of all the checks in docker around rm/stop/etc.
func (s *State) exit(t Transition, restarting bool) {
	s.Running = restarting
	s.Restarting = restarting
	s.Paused = false
	s.Pid = 0
	s.FinishedAt = t.ExitedAt
	if s.FinishedAt.IsZero() {
		s.FinishedAt = time.Now().UTC()
	}
	s.ExitCode = t.ExitCode
	if t.Error != nil {
		s.SetError(t.Error)
	}
	s.notifyAndClear(&s.stopWaiters)
}

// change applies a change to the state and returns it.
// If the change sets a new status or removes the container, it records the
// change and wakes status watchers.
// A removed container's state never changes again for status watchers.
// The caller must hold the state lock.
func (s *State) change(apply func()) StateChange {
	wasDeleted := s.deleted
	prev := s.Snapshot()
	apply()
	change := StateChange{Previous: prev, Current: s.Snapshot()}
	if wasDeleted || (!s.deleted && change.Current.Status == prev.Status) {
		return change
	}
	s.lastChange = change
	s.statusChangedAt = time.Now().UTC()
	if s.statusChanged != nil {
		close(s.statusChanged)
		s.statusChanged = nil
	}
	return change
}

// Snapshot returns a copy of the container's state.
// The caller must hold the state lock.
func (s *State) Snapshot() StateSnapshot {
	snap := StateSnapshot{
		Status:     s.State(),
		PID:        s.Pid,
		ExitCode:   s.ExitCode,
		Error:      s.ErrorMsg,
		OOMKilled:  s.OOMKilled,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
	}
	if s.Health != nil {
		snap.Health = s.Health.Status()
	}
	return snap
}
