package container

import (
	"fmt"
	"strings"
)

// ContainerState is a string representation of the container's current state.
//
// It currently is an alias for string, but may become a distinct type in the future.
type ContainerState = string

const (
	StateCreated    ContainerState = "created"    // StateCreated indicates the container is created, but not (yet) started.
	StateRunning    ContainerState = "running"    // StateRunning indicates that the container is running.
	StatePaused     ContainerState = "paused"     // StatePaused indicates that the container's current state is paused.
	StateRestarting ContainerState = "restarting" // StateRestarting indicates that the container is currently restarting.
	StateRemoving   ContainerState = "removing"   // StateRemoving indicates that the container is being removed.
	StateExited     ContainerState = "exited"     // StateExited indicates that the container exited.
	StateDead       ContainerState = "dead"       // StateDead indicates that the container failed to be deleted. Containers in this state are attempted to be cleaned up when the daemon restarts.
)

var validStates = []ContainerState{
	StateCreated, StateRunning, StatePaused, StateRestarting, StateRemoving, StateExited, StateDead,
}

// ValidateContainerState checks if the provided string is a valid
// container [ContainerState].
func ValidateContainerState(s ContainerState) error {
	switch s {
	case StateCreated, StateRunning, StatePaused, StateRestarting, StateRemoving, StateExited, StateDead:
		return nil
	default:
		return errInvalidParameter{error: fmt.Errorf("invalid value for state (%s): must be one of %s", s, strings.Join(validStates, ", "))}
	}
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

// NewStateStatus returns a new StateStatus with the given exit code and error.
func NewStateStatus(exitCode int, err error) StateStatus {
	return StateStatus{
		exitCode: exitCode,
		err:      err,
	}
}
