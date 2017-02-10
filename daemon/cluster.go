package daemon

import (
	apitypes "github.com/docker/docker/api/types"
)

// Cluster is the interface for github.com/docker/docker/daemon/cluster.(*Cluster).
type Cluster interface {
	GetNetwork(input string) (apitypes.NetworkResource, error)
	GetNetworks() ([]apitypes.NetworkResource, error)
	RemoveNetwork(input string) error
}
