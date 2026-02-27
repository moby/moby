package network

import (
	"context"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/internal/filters"
	dnetwork "github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	GetNetworks(context.Context, dnetwork.Filter, backend.NetworkListConfig) ([]network.Inspect, error)
	GetNetworkSummaries(context.Context, dnetwork.Filter) ([]network.Summary, error)
	CreateNetwork(context.Context, network.CreateRequest) (*network.CreateResponse, error)
	ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(ctx context.Context, containerName string, networkName string, force bool) error
	DeleteNetwork(ctx context.Context, networkID string) error
	NetworkPrune(ctx context.Context, pruneFilters filters.Args) (*network.PruneReport, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster network specific functionality.
type ClusterBackend interface {
	GetNetworks(ctx context.Context, filter dnetwork.Filter, withStatus bool) ([]network.Inspect, error)
	GetNetworkSummaries(context.Context, dnetwork.Filter) ([]network.Summary, error)
	GetNetwork(ctx context.Context, name string, withStatus bool) (network.Inspect, error)
	GetNetworksByName(context.Context, string) ([]network.Network, error)
	CreateNetwork(context.Context, network.CreateRequest) (string, error)
	RemoveNetwork(ctx context.Context, name string) error
}
