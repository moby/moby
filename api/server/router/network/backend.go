package network // import "github.com/docker/docker/api/server/router/network"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/libnetwork"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	FindNetwork(idName string) (libnetwork.Network, error)
	GetNetworks() []libnetwork.Network
	CreateNetwork(nc types.NetworkCreateRequest) (*types.NetworkCreateResponse, error)
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error
	DeleteNetwork(networkID string) error
	NetworksPrune(ctx context.Context, pruneFilters filters.Args) (*types.NetworksPruneReport, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster network specific functionality.
type ClusterBackend interface {
	GetNetworks() ([]types.NetworkResource, error)
	GetNetwork(name string) (types.NetworkResource, error)
	GetNetworksByName(name string) ([]types.NetworkResource, error)
	CreateNetwork(nc types.NetworkCreateRequest) (string, error)
	RemoveNetwork(name string) error
}
