package client

import "github.com/moby/moby/api/types/swarm"

// NodeUpdateOptions holds parameters to update nodes with.
type NodeUpdateOptions struct {
	Version swarm.Version
	Node    swarm.NodeSpec
}
