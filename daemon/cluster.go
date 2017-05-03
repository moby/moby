package daemon

import (
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

// Cluster is the interface for github.com/docker/docker/daemon/cluster.(*Cluster).
type Cluster interface {
	ClusterStatus
	NetworkManager
	ClusterIntrospector
}

// ClusterStatus interface provides information about the Swarm status of the Cluster
type ClusterStatus interface {
	IsAgent() bool
	IsManager() bool
}

// NetworkManager provides methods to manage networks
type NetworkManager interface {
	GetNetwork(input string) (apitypes.NetworkResource, error)
	GetNetworks() ([]apitypes.NetworkResource, error)
	RemoveNetwork(input string) error
}

// ClusterIntrospector provides methods for introspection system
type ClusterIntrospector interface {
	// GetTask returns a task by an ID.
	GetTask(input string) (swarm.Task, error)
	// GetService returns a service based on an ID or name.
	GetService(input string, insertDefaults bool) (swarm.Service, error)
	// GetNode returns a node based on an ID or name.
	GetNode(input string) (swarm.Node, error)
}
