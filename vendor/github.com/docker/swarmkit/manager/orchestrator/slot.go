package orchestrator

import (
	"github.com/docker/swarmkit/api"
)

// Slot is a list of the running tasks occupying a certain slot. Generally this
// will only be one task, but some rolling update situations involve
// temporarily having two running tasks in the same slot. Note that this use of
// "slot" is more generic than the Slot number for replicated services - a node
// is also considered a slot for global services.
type Slot []*api.Task

// SlotTuple identifies a unique slot, in the broad sense described above. It's
// a combination of either a service ID and a slot number (replicated services),
// or a service ID and a node ID (global services).
type SlotTuple struct {
	Slot      uint64 // unset for global service tasks
	ServiceID string
	NodeID    string // unset for replicated service tasks
}
