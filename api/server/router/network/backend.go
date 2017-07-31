package network

import (
	"golang.org/x/net/context"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
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
	DeleteNetwork(name string) error
	NetworksPrune(ctx context.Context, pruneFilters filters.Args) (*types.NetworksPruneReport, error)
}
