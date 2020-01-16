package daemon // import "github.com/moby/moby/daemon"

import (
	apitypes "github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/filters"
	lncluster "github.com/docker/libnetwork/cluster"
)

// Cluster is the interface for github.com/moby/moby/daemon/cluster.(*Cluster).
type Cluster interface {
	ClusterStatus
	NetworkManager
	SendClusterEvent(event lncluster.ConfigEventType)
}

// ClusterStatus interface provides information about the Swarm status of the Cluster
type ClusterStatus interface {
	IsAgent() bool
	IsManager() bool
}

// NetworkManager provides methods to manage networks
type NetworkManager interface {
	GetNetwork(input string) (apitypes.NetworkResource, error)
	GetNetworks(filters.Args) ([]apitypes.NetworkResource, error)
	RemoveNetwork(input string) error
}
