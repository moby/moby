package provider

import (
	"github.com/docker/docker/api/types/network"
	"golang.org/x/net/context"
)

const (
	// ClusterEventSocketChange control socket changed
	ClusterEventSocketChange = iota
	// ClusterEventNodeReady cluster node in ready state
	ClusterEventNodeReady
	// ClusterEventNodeLeave node is leaving the cluster
	ClusterEventNodeLeave
	// ClusterEventNetworkKeysAvailable network keys correctly configured in the networking layer
	ClusterEventNetworkKeysAvailable
)

// ClusterConfigEventType type of the event produced by the cluster
type ClusterConfigEventType uint8

// Cluster provides clustering config details
type Cluster interface {
	IsManager() bool
	IsAgent() bool
	GetLocalAddress() string
	GetListenAddress() string
	GetAdvertiseAddress() string
	GetDataPathAddress() string
	GetRemoteAddressList() []string
	ListenClusterEvents() <-chan ClusterConfigEventType
	AttachNetwork(string, string, []string) (*network.NetworkingConfig, error)
	DetachNetwork(string, string) error
	UpdateAttachment(string, string, *network.NetworkingConfig) error
	WaitForDetachment(context.Context, string, string, string, string) error
}
