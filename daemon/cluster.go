package daemon

import (
	"context"

	"github.com/moby/moby/api/types/network"
	lncluster "github.com/moby/moby/v2/daemon/libnetwork/cluster"
	dnetwork "github.com/moby/moby/v2/daemon/network"
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
	GetNetwork(ctx context.Context, input string, withStatus bool) (network.Inspect, error)
	GetNetworks(ctx context.Context, filter dnetwork.Filter, withStatus bool) ([]network.Inspect, error)
	RemoveNetwork(ctx context.Context, input string) error
}
