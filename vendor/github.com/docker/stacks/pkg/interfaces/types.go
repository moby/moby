package interfaces

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

const (
	// StackEventType is the value of Type in an events.Message for stacks
	StackEventType = "stack"
	// StackLabel is a label on objects indicating the stack that it belongs to
	StackLabel = "com.docker.stacks.stack_id"
)

// SwarmStack represents a Stack with all of its elements converted to Engine
// API types.
// NOTE: SwarmStack is only used internally for reconciliation of Swarm
// stacks and is never exposed via the API.
type SwarmStack struct {
	ID   string
	Meta swarm.Meta
	Spec SwarmStackSpec
}

// SwarmStackSpec represents a StackSpec with all of its elements converted to
// Engine API types.
// NOTE: SwarmStackSpec is only used internally for reconciliation of
// Swarm stacks and is never exposed via the API.
type SwarmStackSpec struct {
	Annotations swarm.Annotations
	Services    []swarm.ServiceSpec
	// Networks is a map of name -> types.NetworkCreate. It's like this because
	// Networks don't have a Spec, they're defined in terms of the
	// NetworkCreate type only.
	Networks map[string]types.NetworkCreate
	Secrets  []swarm.SecretSpec
	Configs  []swarm.ConfigSpec
	// there is no "Volumes" in a SwarmStackSpec -- Swarm has no concept of
	// volumes
}
