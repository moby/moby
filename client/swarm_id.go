package client

import "github.com/moby/moby/api/types/swarm"

// SwarmVersionedID is a struct that contains an ID and a version.
// It is used to identify a swarm resource with a specific version.
// This is used to avoid conflicts when updating a resource.
type SwarmVersionedID struct {
	ID      string
	Version swarm.Version
}
