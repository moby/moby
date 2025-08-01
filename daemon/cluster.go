package daemon

import (
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	lncluster "github.com/moby/moby/v2/daemon/libnetwork/cluster"
)

// Cluster is the interface for [github.com/moby/moby/v2/daemon/cluster.Cluster].
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
	GetNetwork(input string) (network.Inspect, error)
	GetNetworks(filters.Args) ([]network.Inspect, error)
	RemoveNetwork(input string) error
}
