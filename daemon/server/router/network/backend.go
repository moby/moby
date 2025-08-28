package network

import (
	"context"

	"github.com/moby/moby/api/types/network"
	dnetwork "github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	GetNetworks(dnetwork.Filter, backend.NetworkListConfig) ([]network.Inspect, error)
	CreateNetwork(ctx context.Context, nc network.CreateRequest) (*network.CreateResponse, error)
	ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error
	DeleteNetwork(networkID string) error
	NetworksPrune(ctx context.Context, pruneFilters dnetwork.Filter) (*network.PruneReport, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster network specific functionality.
type ClusterBackend interface {
	GetNetworks(dnetwork.Filter) ([]network.Inspect, error)
	GetNetwork(name string) (network.Inspect, error)
	GetNetworksByName(name string) ([]network.Inspect, error)
	CreateNetwork(nc network.CreateRequest) (string, error)
	RemoveNetwork(name string) error
}
