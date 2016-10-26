// Package naming centralizes the naming of SwarmKit objects.
package naming

import (
	"fmt"

	"github.com/docker/swarmkit/api"
)

// Task returns the task name from Annotations.Name,
// and, in case Annotations.Name is missing, fallback
// to construct the name from othere information.
func Task(t *api.Task) string {
	if t.Annotations.Name != "" {
		// if set, use the container Annotations.Name field, set in the orchestrator.
		return t.Annotations.Name
	}

	slot := fmt.Sprint(t.Slot)
	if slot == "" || t.Slot == 0 {
		// when no slot id is assigned, we assume that this is node-bound task.
		slot = t.NodeID
	}

	// fallback to service.instance.id.
	return fmt.Sprintf("%s.%s.%s", t.ServiceAnnotations.Name, slot, t.ID)
}

// TODO(stevvooe): Consolidate "Hostname" style validation here.
