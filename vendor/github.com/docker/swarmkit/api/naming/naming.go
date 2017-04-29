// Package naming centralizes the naming of SwarmKit objects.
package naming

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/swarmkit/api"
)

var (
	errUnknownRuntime = errors.New("unrecognized runtime")
)

// Task returns the task name from Annotations.Name,
// and, in case Annotations.Name is missing, fallback
// to construct the name from other information.
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

// Runtime returns the runtime name from a given spec.
func Runtime(t api.TaskSpec) (string, error) {
	switch r := t.GetRuntime().(type) {
	case *api.TaskSpec_Attachment:
		return "attachment", nil
	case *api.TaskSpec_Container:
		return "container", nil
	case *api.TaskSpec_Generic:
		return strings.ToLower(r.Generic.Kind), nil
	default:
		return "", errUnknownRuntime
	}
}
