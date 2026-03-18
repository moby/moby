package container

import (
	"fmt"
	"strings"
)

// ContainerState is a string representation of the container's current state.
type ContainerState string

const (
	StateCreated    ContainerState = "created"    // StateCreated indicates the container is created, but not (yet) started.
	StateRunning    ContainerState = "running"    // StateRunning indicates that the container is running.
	StatePaused     ContainerState = "paused"     // StatePaused indicates that the container's current state is paused.
	StateRestarting ContainerState = "restarting" // StateRestarting indicates that the container is currently restarting.
	StateRemoving   ContainerState = "removing"   // StateRemoving indicates that the container is being removed.
	StateExited     ContainerState = "exited"     // StateExited indicates that the container exited.
	StateDead       ContainerState = "dead"       // StateDead indicates that the container failed to be deleted. Containers in this state are attempted to be cleaned up when the daemon restarts.
)

var validStates = []string{
	string(StateCreated),
	string(StateRunning),
	string(StatePaused),
	string(StateRestarting),
	string(StateRemoving),
	string(StateExited),
	string(StateDead),
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
